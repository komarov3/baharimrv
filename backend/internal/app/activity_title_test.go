package app_test

import (
	"net/http"
	"strings"
	"testing"
)

// A name is the manager's own label. It changes no attestation, so the tests below
// care only that it round-trips, that writing it is manager-or-admin work, and that an
// empty name is a legal way to clear one.

func TestActivityTitleRoundTripsThroughCreateDetailAndList(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()

	// An activity created without a name reports none.
	detail := f.must(http.MethodGet, "/api/v1/activities/"+w.activity+"/detail", w.manager.token, nil, http.StatusOK)
	if value, ok := detail.body["title"]; ok && value != "" {
		t.Fatalf("an unnamed activity must carry no title, got %v", value)
	}

	renamed := f.must(http.MethodPatch, "/api/v1/activities/"+w.activity, w.manager.token, map[string]any{
		"title": " Mikoko first cycle ",
	}, http.StatusOK)
	if renamed.str("title") != "Mikoko first cycle" {
		t.Errorf("surrounding space is dropped: %q", renamed.str("title"))
	}

	detail = f.must(http.MethodGet, "/api/v1/activities/"+w.activity+"/detail", w.manager.token, nil, http.StatusOK)
	if detail.str("title") != "Mikoko first cycle" {
		t.Errorf("detail title = %q", detail.str("title"))
	}
	if detail.str("status") == "" {
		t.Error("the rename must not disturb the verification status")
	}

	items := f.must(http.MethodGet, "/api/v1/projects/"+w.projectID+"/activities", w.manager.token, nil, http.StatusOK).list("items")
	if len(items) != 1 {
		t.Fatalf("expected one activity, got %d", len(items))
	}
	item, _ := items[0].(map[string]any)
	if item["title"] != "Mikoko first cycle" {
		t.Errorf("list item title = %v", item["title"])
	}

	cleared := f.must(http.MethodPatch, "/api/v1/activities/"+w.activity, w.manager.token, map[string]any{
		"title": "",
	}, http.StatusOK)
	if cleared.str("title") != "" {
		t.Errorf("an empty title clears the name, got %q", cleared.str("title"))
	}
}

func TestActivityTitleWritesAreManagerWork(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()

	if got := f.call(http.MethodPatch, "/api/v1/activities/"+w.activity, w.monitor.token, map[string]any{"title": "Forged label"}).status; got != http.StatusForbidden {
		t.Errorf("monitor rename = %d, want 403", got)
	}
	if got := f.call(http.MethodPatch, "/api/v1/activities/"+w.activity, "", map[string]any{"title": "No session"}).status; got != http.StatusUnauthorized {
		t.Errorf("anonymous rename = %d, want 401", got)
	}

	detail := f.must(http.MethodGet, "/api/v1/activities/"+w.activity+"/detail", w.manager.token, nil, http.StatusOK)
	if value, ok := detail.body["title"]; ok && value != "" {
		t.Fatalf("a refused rename must leave the activity unnamed, got %v", value)
	}
}

func TestActivityTitleInputIsChecked(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()

	// A body without the field is not the same as a request to clear the name.
	if got := f.call(http.MethodPatch, "/api/v1/activities/"+w.activity, w.manager.token, map[string]any{}).status; got != http.StatusUnprocessableEntity {
		t.Errorf("missing title = %d, want 422", got)
	}
	oversized := f.call(http.MethodPatch, "/api/v1/activities/"+w.activity, w.manager.token, map[string]any{
		"title": strings.Repeat("a", 121),
	})
	if oversized.status != http.StatusUnprocessableEntity {
		t.Errorf("121-character title = %d, want 422: %s", oversized.status, oversized.raw)
	}
	if got := f.call(http.MethodPatch, "/api/v1/activities/"+"00000000-0000-0000-0000-000000000000", w.manager.token, map[string]any{"title": "Elsewhere"}).status; got != http.StatusNotFound {
		t.Errorf("unknown activity = %d, want 404", got)
	}
}

func TestActivityTitleIsStoredAtCreation(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()

	created := f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/activities", w.manager.token, map[string]any{
		"plotId": w.plotID, "activityType": "restoration", "methodologyVersion": "1.0", "title": "Long rains cycle",
	}, http.StatusCreated)
	if created.str("title") != "Long rains cycle" {
		t.Errorf("create response title = %q", created.str("title"))
	}

	rejected := f.call(http.MethodPost, "/api/v1/projects/"+w.projectID+"/activities", w.manager.token, map[string]any{
		"plotId": w.plotID, "activityType": "restoration", "methodologyVersion": "1.0", "title": strings.Repeat("b", 200),
	})
	if rejected.status != http.StatusUnprocessableEntity {
		t.Errorf("oversized title at creation = %d, want 422", rejected.status)
	}
}
