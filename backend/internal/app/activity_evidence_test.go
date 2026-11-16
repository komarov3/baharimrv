package app_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/baharilink/baharimrv/internal/models"
)

func photoBytes(t *testing.T, red uint8) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: red, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func (f *fixture) activityPhoto(activityID, token, kind, hash, mediaType string, content []byte) response {
	f.t.Helper()
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)
	part, err := writer.CreateFormFile("file", "photo.png")
	if err != nil {
		f.t.Fatal(err)
	}
	if _, err := part.Write(content); err != nil {
		f.t.Fatal(err)
	}
	for key, value := range map[string]string{"kind": kind, "sha256": hash, "mediaType": mediaType} {
		if err := writer.WriteField(key, value); err != nil {
			f.t.Fatal(err)
		}
	}
	if err := writer.Close(); err != nil {
		f.t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/api/v1/activities/"+activityID+"/evidence", &buf)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+token)
	recorder := httptest.NewRecorder()
	f.app.Handler.ServeHTTP(recorder, req)
	result := response{status: recorder.Code, raw: recorder.Body.Bytes()}
	_ = json.Unmarshal(result.raw, &result.body)
	return result
}

func photoHash(data []byte) string {
	digest := sha256.Sum256(data)
	return hex.EncodeToString(digest[:])
}

func TestActivityPhotosPersistSeparatelyAndAreRestricted(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	data := photoBytes(t, 1)
	created := f.activityPhoto(w.activity, w.manager.token, "photo", photoHash(data), "image/png", data)
	if created.status != http.StatusCreated {
		t.Fatalf("upload: %d %s", created.status, created.raw)
	}
	if _, exposed := created.body["storageURI"]; exposed {
		t.Fatal("storage path exposed")
	}
	repeated := f.activityPhoto(w.activity, w.manager.token, "photo", photoHash(data), "image/png", data)
	if repeated.status != http.StatusCreated || repeated.str("id") != created.str("id") {
		t.Fatal("retry duplicated photo")
	}
	detail := f.must(http.MethodGet, "/api/v1/activities/"+w.activity+"/detail", w.manager.token, nil, 200)
	if len(detail.list("activityEvidence")) != 1 {
		t.Fatal("photo absent from report")
	}
	mediaPath := "/api/v1/activities/" + w.activity + "/evidence/" + created.str("id")
	downloaded := f.must(http.MethodGet, mediaPath, w.manager.token, nil, 200)
	if !bytes.Equal(downloaded.raw, data) {
		t.Fatal("download differs from uploaded photo")
	}
	outsider := f.register(models.RoleManager, "outsider")
	f.must(http.MethodGet, mediaPath, outsider.token, nil, 403)
	denied := f.activityPhoto(w.activity, w.monitor.token, "photo", photoHash(data), "image/png", data)
	if denied.status != 403 {
		t.Fatalf("monitor upload: %d", denied.status)
	}
	var count int64
	f.app.DB.Model(&models.EvidenceObject{}).Count(&count)
	if count != 0 {
		t.Fatal("activity photo entered attested evidence")
	}
	f.must(http.MethodPost, "/api/v1/projects/"+w.projectID+"/archive", w.manager.token, map[string]any{}, 200)
	denied = f.activityPhoto(w.activity, w.manager.token, "photo", photoHash(data), "image/png", data)
	if denied.status < 400 {
		t.Fatal("archived project accepted upload")
	}
}

func TestActivityPhotoValidationAndLimit(t *testing.T) {
	f := newFixture(t)
	w := f.newWork()
	data := photoBytes(t, 1)
	cases := []struct {
		kind, hash, media string
		data              []byte
	}{
		{"photo", strings.Repeat("0", 64), "image/png", data},
		{"voice", photoHash(data), "image/png", data},
		{"photo", photoHash(data), "image/jpeg", data},
		{"photo", photoHash([]byte("<svg/>")), "image/svg+xml", []byte("<svg/>")},
		{"photo", photoHash(nil), "image/png", nil},
		{"photo", "", "image/png", make([]byte, 5*1024*1024+1)},
	}
	for i, c := range cases {
		got := f.activityPhoto(w.activity, w.manager.token, c.kind, c.hash, c.media, c.data)
		if got.status < 400 {
			t.Fatalf("case %d accepted", i)
		}
	}
	for i := 0; i < 10; i++ {
		b := photoBytes(t, uint8(i))
		got := f.activityPhoto(w.activity, w.manager.token, "photo", photoHash(b), "image/png", b)
		if got.status != 201 {
			t.Fatalf("photo %d: %d %s", i, got.status, got.raw)
		}
	}
	extra := photoBytes(t, 11)
	got := f.activityPhoto(w.activity, w.manager.token, "photo", photoHash(extra), "image/png", extra)
	if got.status < 400 {
		t.Fatal("eleventh photo accepted")
	}
	items := f.must(http.MethodGet, "/api/v1/activities/"+w.activity+"/evidence", w.manager.token, nil, 200).list("items")
	if len(items) != 10 {
		t.Fatalf("got %d photos", len(items))
	}
}
