package mrv

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"strings"

	"github.com/baharilink/baharimrv/internal/domain"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

const ActivityPhotoMaxBytes = 5 * 1024 * 1024
const ActivityPhotoMaxCount = 10

func (s *Service) UploadActivityEvidence(userID, activityID string, manifest EvidenceManifestItem, upload io.ReadCloser) (models.ActivityEvidence, error) {
	defer upload.Close()
	activity, _, err := s.Activity(activityID)
	if err != nil {
		return models.ActivityEvidence{}, err
	}
	if err := s.AssertProjectManager(userID, activity.ProjectID); err != nil {
		return models.ActivityEvidence{}, err
	}
	if err := s.assertProjectWritable(activity.ProjectID); err != nil {
		return models.ActivityEvidence{}, err
	}
	if manifest.Kind != "photo" {
		return models.ActivityEvidence{}, domain.Invalid("activity evidence must be a photo")
	}
	data, err := io.ReadAll(io.LimitReader(upload, ActivityPhotoMaxBytes+1))
	if err != nil {
		return models.ActivityEvidence{}, domain.Invalid("the photo could not be read")
	}
	if len(data) == 0 || len(data) > ActivityPhotoMaxBytes {
		return models.ActivityEvidence{}, domain.Invalid("photos must be non-empty and no larger than 5 MB")
	}
	config, format, err := image.DecodeConfig(bytes.NewReader(data))
	if err != nil || config.Width <= 0 || config.Height <= 0 || int64(config.Width)*int64(config.Height) > 100_000_000 {
		return models.ActivityEvidence{}, domain.Invalid("use a JPEG, PNG or GIF photo no larger than 100 megapixels")
	}
	mediaType := "image/" + format
	if manifest.MediaType != "" && manifest.MediaType != mediaType {
		return models.ActivityEvidence{}, domain.Invalid("photo media type does not match its contents")
	}
	digest := sha256.Sum256(data)
	actual := hex.EncodeToString(digest[:])
	if actual != strings.ToLower(strings.TrimSpace(manifest.SHA256)) {
		return models.ActivityEvidence{}, domain.Invalid("the uploaded bytes do not match the declared sha256")
	}
	var object models.ActivityEvidence
	err = s.db.Transaction(func(tx *gorm.DB) error {
		// A write lock serializes the count and insertion across both database drivers.
		if err := tx.Model(&models.ConservationActivity{}).Where("id = ?", activityID).UpdateColumn("id", activityID).Error; err != nil {
			return err
		}
		err := tx.Where("activity_id = ? AND sha256 = ?", activityID, actual).First(&object).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}
		var count int64
		if err := tx.Model(&models.ActivityEvidence{}).Where("activity_id = ?", activityID).Count(&count).Error; err != nil {
			return err
		}
		if count >= ActivityPhotoMaxCount {
			return domain.Reject("an activity can have at most 10 photos")
		}
		id := uuid.NewString()
		uri, hash, size, err := s.evidence.Put("activity-"+activityID, id, actual, bytes.NewReader(data))
		if err != nil {
			return evidenceError(err)
		}
		object = models.ActivityEvidence{ID: id, ActivityID: activityID, Kind: "photo", StorageURI: uri, SHA256: hash, MediaType: mediaType, SizeBytes: size, CreatedAt: s.now().UTC()}
		return tx.Create(&object).Error
	})
	return object, err
}

func (s *Service) ActivityEvidenceFor(activityID string) ([]models.ActivityEvidence, error) {
	objects := make([]models.ActivityEvidence, 0)
	err := s.db.Where("activity_id = ?", activityID).Order("created_at").Order("id").Find(&objects).Error
	return objects, err
}
