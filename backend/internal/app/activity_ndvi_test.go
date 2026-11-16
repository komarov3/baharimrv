package app_test

import (
	"context"
	"encoding/json"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/satellite"
)

func createNdviPlanting(f *fixture, w work, start string) response {
	f.t.Helper()
	body := map[string]any{
		"plotId": w.plotID, "activityType": "planting", "methodologyVersion": "1.0",
		"title": " Long rains planting ",
	}
	if start != "" {
		body["reportingPeriodStart"] = start
	}
	return f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/activities", w.manager.token, body, http.StatusCreated)
}

func readNdviRequest(t *testing.T, f *fixture, activityID string) models.ActivityNdviRequest {
	t.Helper()
	var request models.ActivityNdviRequest
	if err := f.app.DB.First(&request, "activity_id = ?", activityID).Error; err != nil {
		t.Fatalf("read durable NDVI request: %v", err)
	}
	return request
}

func assertWaitingNdviJSON(t *testing.T, result response, center time.Time) {
	t.Helper()
	var view satellite.ActivityNdviView
	if err := json.Unmarshal(result.raw, &view); err != nil {
		t.Fatal(err)
	}
	if view.Status != "waiting" || view.JobID != "" || view.Error != "" || view.CenterAt == nil || !view.CenterAt.Equal(center) || view.WindowStart == nil || !view.WindowStart.Equal(center.Add(-15*24*time.Hour)) || view.WindowEnd == nil || !view.WindowEnd.Equal(center.Add(15*24*time.Hour)) {
		t.Errorf("waiting contract = %s", result.raw)
	}
	for _, key := range []string{"observation", "baseline", "latestPass", "changeAbs", "changePct"} {
		value, present := result.body[key]
		if !present || value != nil {
			t.Errorf("waiting %s = %v (present %v), want explicit null", key, value, present)
		}
	}
}

func TestActivityNdviAPICreationQueuesDateSnapshot(t *testing.T) {
	f := newFixture(t)
	w := f.newWork() // This fixture's existing activity is restoration, not planting.
	var count int64
	if err := f.app.DB.Model(&models.ActivityNdviRequest{}).Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatal("creating a restoration activity queued a planting baseline")
	}
	for _, tc := range []struct{ name, start string }{
		{"creation_timestamp", ""},
		{"supplied_timestamp", "2026-01-17T14:32:19.123+03:00"},
		{"supplied_date", "2026-01-17"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			created := createNdviPlanting(f, w, tc.start)
			var activity models.ConservationActivity
			if err := json.Unmarshal(created.raw, &activity); err != nil {
				t.Fatal(err)
			}
			if activity.ID == "" || activity.Title != "Long rains planting" || activity.ManagerID != w.manager.id {
				t.Fatalf("created planting = %s", created.raw)
			}
			center := activity.CreatedAt
			if tc.start != "" {
				layout := time.RFC3339Nano
				if tc.name == "supplied_date" {
					layout = "2006-01-02"
				}
				var err error
				center, err = time.Parse(layout, tc.start)
				if err != nil {
					t.Fatal(err)
				}
				if !activity.ReportingPeriodStart.Equal(center) {
					t.Errorf("supplied reporting timestamp lost: %s, want %s", activity.ReportingPeriodStart, center)
				}
			}
			if center.IsZero() {
				t.Fatal("creation did not return a usable capture timestamp")
			}
			request := readNdviRequest(t, f, activity.ID)
			if request.Status != "waiting" || request.JobID != "" || request.ObservationID != "" || !request.CenterAt.Equal(center) || request.RequestedBy != w.manager.id || request.ProjectID != w.projectID || request.PlotID != w.plotID || len(request.AOI) == 0 || request.GeometryHash == "" {
				t.Errorf("durable capture does not match creation: %+v", request)
			}
			ndvi := f.must(http.MethodGet, "/api/v1/activities/"+activity.ID+"/ndvi", w.manager.token, nil, http.StatusOK)
			assertWaitingNdviJSON(t, ndvi, center)
			detail := f.must(http.MethodGet, "/api/v1/activities/"+activity.ID+"/detail", w.manager.token, nil, http.StatusOK)
			if !reflect.DeepEqual(detail.body["ndvi"], ndvi.body) || detail.str("title") != activity.Title {
				t.Errorf("detail disagrees with the NDVI endpoint: %s", detail.raw)
			}
		})
	}
	for _, model := range []any{&models.SatelliteRefreshJob{}, &models.SatelliteObservation{}} {
		if err := f.app.DB.Model(model).Count(&count).Error; err != nil {
			t.Fatal(err)
		}
		if count != 0 {
			t.Errorf("activity creation synchronously wrote %d %T rows", count, model)
		}
	}
}

func TestActivityNdviAPIRecaptureAndAccess(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	created := createNdviPlanting(f, w, "")
	activityID := created.str("id")
	path := "/api/v1/activities/" + activityID + "/ndvi"
	original := readNdviRequest(t, f, activityID)

	// A duplicate tap while still waiting is a no-op, not another request.
	waiting := f.must(http.MethodPost, path+"/recapture", w.manager.token, nil, http.StatusAccepted)
	assertWaitingNdviJSON(t, waiting, original.CenterAt)
	if got := readNdviRequest(t, f, activityID); !reflect.DeepEqual(got, original) {
		t.Errorf("duplicate waiting recapture changed the request: %+v", got)
	}
	finished, err := f.app.Satellite.ProcessRefreshJobs(context.Background())
	if err != nil || finished != 1 {
		t.Fatalf("process sandbox baseline: %d, %v", finished, err)
	}
	complete := f.must(http.MethodGet, path, w.manager.token, nil, http.StatusOK)
	if complete.str("status") != "synthetic" || complete.str("observation", "adapterMode") != "sandbox" || complete.str("observation", "quality") != "synthetic" || complete.str("observation", "id") == "" {
		t.Fatalf("completed API capture = %s", complete.raw)
	}
	detail := f.must(http.MethodGet, "/api/v1/activities/"+activityID+"/detail", w.manager.token, nil, http.StatusOK)
	if !reflect.DeepEqual(detail.body["ndvi"], complete.body) {
		t.Error("detail did not expose the completed server capture")
	}
	beforeDenied := readNdviRequest(t, f, activityID)
	outsider := f.register(models.RoleManager, "ndvi-outsider")
	f.must(http.MethodGet, path, w.monitor.token, nil, http.StatusOK)
	f.must(http.MethodGet, path, outsider.token, nil, http.StatusForbidden)
	f.must(http.MethodGet, "/api/v1/activities/"+activityID+"/detail", outsider.token, nil, http.StatusForbidden)
	f.must(http.MethodGet, path, "", nil, http.StatusUnauthorized)
	for _, denied := range []struct {
		token  string
		status int
	}{{w.monitor.token, http.StatusForbidden}, {outsider.token, http.StatusForbidden}, {"", http.StatusUnauthorized}} {
		f.must(http.MethodPost, path+"/recapture", denied.token, nil, denied.status)
	}
	if got := readNdviRequest(t, f, activityID); !reflect.DeepEqual(got, beforeDenied) {
		t.Errorf("denied recapture changed the stored request: %+v", got)
	}
	f.must(http.MethodPost, "/api/v1/activities/"+w.activity+"/ndvi/recapture", w.manager.token, nil, http.StatusUnprocessableEntity)
	var otherRequests int64
	if err := f.app.DB.Model(&models.ActivityNdviRequest{}).Where("activity_id = ?", w.activity).Count(&otherRequests).Error; err != nil {
		t.Fatal(err)
	}
	if otherRequests != 0 {
		t.Error("rejected restoration recapture created a planting request")
	}

	// Recapture after completion resets the queue but preserves the original date,
	// not the default reporting-period start three months before creation.
	recaptured := f.must(http.MethodPost, path+"/recapture", w.manager.token, nil, http.StatusAccepted)
	assertWaitingNdviJSON(t, recaptured, original.CenterAt)
	request := readNdviRequest(t, f, activityID)
	if request.Status != "waiting" || request.JobID != "" || request.ObservationID != "" || !request.CenterAt.Equal(original.CenterAt) || request.GeometryHash != original.GeometryHash {
		t.Errorf("manager recapture did not reset the durable request: %+v", request)
	}
	f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/archive", w.manager.token, map[string]any{}, http.StatusOK)
	f.must(http.MethodPost, path+"/recapture", w.manager.token, nil, http.StatusConflict)
	if got := readNdviRequest(t, f, activityID); !reflect.DeepEqual(got, request) {
		t.Errorf("archived recapture changed the stored request: %+v", got)
	}
}

func TestActivityTitlePreservesCaptureTimestampsAndNdviRequest(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	created := createNdviPlanting(f, w, "2026-01-17T14:32:19.123+03:00")
	activityID := created.str("id")
	// Use a distinct historical UpdatedAt so an accidental ORM timestamp update
	// cannot pass merely because create and rename happened in the same tick.
	captureTime := time.Date(2026, 1, 18, 7, 4, 3, 123000000, time.UTC)
	if err := f.app.DB.Model(&models.ConservationActivity{}).Where("id = ?", activityID).UpdateColumn("updated_at", captureTime).Error; err != nil {
		t.Fatal(err)
	}
	var before models.ConservationActivity
	if err := f.app.DB.First(&before, "id = ?", activityID).Error; err != nil {
		t.Fatal(err)
	}
	request := readNdviRequest(t, f, activityID)
	updated := f.must(http.MethodPatch, "/api/v1/activities/"+activityID, w.manager.token, map[string]any{"title": " Renamed planting "}, http.StatusOK)
	if updated.str("id") != activityID || updated.str("title") != "Renamed planting" {
		t.Errorf("rename response = %s", updated.raw)
	}
	var after models.ConservationActivity
	if err := f.app.DB.First(&after, "id = ?", activityID).Error; err != nil {
		t.Fatal(err)
	}
	if after.Title != "Renamed planting" || !after.CreatedAt.Equal(before.CreatedAt) || !after.UpdatedAt.Equal(before.UpdatedAt) || !after.ReportingPeriodStart.Equal(before.ReportingPeriodStart) || !after.ReportingPeriodEnd.Equal(before.ReportingPeriodEnd) || after.Status != before.Status || after.CurrentEventID != before.CurrentEventID {
		t.Errorf("rename changed capture/provenance fields: %+v", after)
	}
	if got := readNdviRequest(t, f, activityID); !reflect.DeepEqual(got, request) {
		t.Errorf("rename rewrote the NDVI date/geometry request: %+v", got)
	}
}
