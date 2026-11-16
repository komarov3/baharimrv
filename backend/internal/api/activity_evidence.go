package api

import (
	"net/http"
	"strings"

	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/baharilink/baharimrv/internal/mrv"
	"github.com/gin-gonic/gin"
)

func (r *router) uploadActivityEvidence(c *gin.Context) {
	file, err := c.FormFile("file")
	if c.Request.MultipartForm != nil {
		defer c.Request.MultipartForm.RemoveAll()
	}
	if err != nil {
		fail(c, domain.Invalid("a multipart photo field named file is required"))
		return
	}
	if file.Size > mrv.ActivityPhotoMaxBytes {
		fail(c, domain.Invalid("photos must be no larger than 5 MB"))
		return
	}
	upload, err := file.Open()
	if err != nil {
		fail(c, domain.Invalid("the photo could not be read"))
		return
	}
	mediaType := strings.TrimSpace(c.PostForm("mediaType"))
	if mediaType == "" {
		mediaType = file.Header.Get("Content-Type")
	}
	if mediaType == "application/octet-stream" {
		mediaType = ""
	}
	object, err := r.projects.UploadActivityEvidence(c.GetString("user_id"), c.Param("id"), mrv.EvidenceManifestItem{
		Kind: strings.TrimSpace(c.PostForm("kind")), SHA256: strings.TrimSpace(c.PostForm("sha256")), MediaType: mediaType,
	}, upload)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusCreated, object)
}

func (r *router) activityEvidence(c *gin.Context) {
	if err := r.projects.AssertActivityAccess(c.GetString("user_id"), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	objects, err := r.projects.ActivityEvidenceFor(c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": objects})
}

func (r *router) activityEvidenceMedia(c *gin.Context) {
	var object models.ActivityEvidence
	if err := r.db.Where("id = ? AND activity_id = ?", c.Param("evidenceId"), c.Param("id")).First(&object).Error; err != nil {
		fail(c, mrv.ErrNotFound)
		return
	}
	if err := r.projects.AssertActivityAccess(c.GetString("user_id"), object.ActivityID); err != nil {
		fail(c, err)
		return
	}
	body, size, err := r.evidence.Open(object.StorageURI)
	if err != nil {
		fail(c, err)
		return
	}
	defer body.Close()
	c.DataFromReader(http.StatusOK, size, object.MediaType, body, map[string]string{"Cache-Control": "private, no-store", "X-Content-Type-Options": "nosniff"})
}
