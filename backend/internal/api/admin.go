package api

import (
	"net/http"
	"time"

	"github.com/baharilink/baharimrv/internal/httpx"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func (r *router) adminOverview(c *gin.Context) {
	type counts struct {
		Users              int64 `json:"users"`
		ActiveUsers        int64 `json:"activeUsers"`
		PendingIdentities  int64 `json:"pendingIdentities"`
		FailedIdentities   int64 `json:"failedIdentities"`
		Projects           int64 `json:"projects"`
		Activities         int64 `json:"activities"`
		VerifiedActivities int64 `json:"verifiedActivities"`
		VerificationQueue  int64 `json:"verificationQueue"`
		AnchorsPending     int64 `json:"anchorsPending"`
		AnchorsFailed      int64 `json:"anchorsFailed"`
		Proofs             int64 `json:"proofs"`
		AuditEvents        int64 `json:"auditEvents"`
	}
	var out counts
	r.db.Model(&models.User{}).Count(&out.Users)
	r.db.Model(&models.User{}).Where("status = ?", models.IdentityActive).Count(&out.ActiveUsers)
	r.db.Model(&models.IdentityProvision{}).Where("state IN ?", []string{"pending", "provisioning"}).Count(&out.PendingIdentities)
	r.db.Model(&models.IdentityProvision{}).Where("state = ?", models.IdentityFailed).Count(&out.FailedIdentities)
	r.db.Model(&models.ConservationProject{}).Count(&out.Projects)
	r.db.Model(&models.ConservationActivity{}).Count(&out.Activities)
	r.db.Model(&models.ConservationActivity{}).Where("status = ?", models.ActivityVerified).Count(&out.VerifiedActivities)
	r.db.Model(&models.VerificationRequest{}).Where("status IN ?", []string{"requested", "in_review"}).Count(&out.VerificationQueue)
	r.db.Model(&models.AnchorAttempt{}).Where("status = ?", "pending").Count(&out.AnchorsPending)
	r.db.Model(&models.AnchorAttempt{}).Where("status = ?", "failed").Count(&out.AnchorsFailed)
	r.db.Model(&models.ProofDocument{}).Count(&out.Proofs)
	r.db.Model(&models.AuditEvent{}).Count(&out.AuditEvents)
	c.JSON(http.StatusOK, out)
}

func (r *router) adminUsers(c *gin.Context) {
	var users []models.User
	if err := r.db.Order("created_at desc").Find(&users).Error; err != nil {
		fail(c, err)
		return
	}
	type row struct {
		models.User
		IdentityState string `json:"identityState"`
	}
	out := make([]row, 0, len(users))
	for _, u := range users {
		var ip models.IdentityProvision
		state := "unknown"
		if err := r.db.Where("user_id = ?", u.ID).First(&ip).Error; err == nil {
			state = ip.State
		} else if err != gorm.ErrRecordNotFound {
			fail(c, err)
			return
		}
		out = append(out, row{User: u, IdentityState: state})
	}
	c.JSON(http.StatusOK, gin.H{"items": out})
}

func (r *router) adminIdentities(c *gin.Context) {
	var items []models.IdentityProvision
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminProjects(c *gin.Context) {
	var items []models.ConservationProject
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminActivities(c *gin.Context) {
	var items []models.ConservationActivity
	if err := r.db.Order("created_at desc").Limit(500).Find(&items).Error; err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminVerificationRequests(c *gin.Context) {
	var items []models.VerificationRequest
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminAnchors(c *gin.Context) {
	var items []models.AnchorAttempt
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminProofs(c *gin.Context) {
	var items []models.ProofDocument
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminAuditEvents(c *gin.Context) {
	var items []models.AuditEvent
	if err := r.db.Order("created_at desc").Limit(1000).Find(&items).Error; err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminIntegrations(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"modes": gin.H{
			"chain":     r.config.ChainMode,
			"satellite": r.config.SatelliteMode,
			"registry":  r.config.RegistryMode,
			"payment":   r.config.PaymentMode,
		},
		"serverTime": time.Now().UTC(),
	})
}


func (r *router) adminEvidence(c *gin.Context) {
	var items []models.EvidenceObject
	if err := r.db.Order("created_at desc").Limit(1000).Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminDataQuality(c *gin.Context) {
	type out struct {
		Activities int64 `json:"activities"`
		EvidenceObjects int64 `json:"evidenceObjects"`
		MonitoringRecords int64 `json:"monitoringRecords"`
	}
	var o out
	r.db.Model(&models.ConservationActivity{}).Count(&o.Activities)
	r.db.Model(&models.EvidenceObject{}).Count(&o.EvidenceObjects)
	r.db.Model(&models.MonitoringRecord{}).Count(&o.MonitoringRecords)
	c.JSON(http.StatusOK, o)
}

func (r *router) adminDisableUser(c *gin.Context) {
	var body struct { Password string `json:"password"` }
	if !httpx.BindJSON(c, &body) { return }
	if !r.auth.CheckPassword(c.GetString("user_id"), body.Password) {
		httpx.Error(c, http.StatusForbidden, "step_up_failed", "Recent password confirmation is required.")
		return
	}
	if err := r.db.Model(&models.User{}).Where("id = ?", c.Param("id")).Update("status", "disabled").Error; err != nil {
		fail(c, err); return
	}
	c.JSON(http.StatusOK, gin.H{"status": "disabled"})
}

func (r *router) adminEnableUser(c *gin.Context) {
	var body struct { Password string `json:"password"` }
	if !httpx.BindJSON(c, &body) { return }
	if !r.auth.CheckPassword(c.GetString("user_id"), body.Password) {
		httpx.Error(c, http.StatusForbidden, "step_up_failed", "Recent password confirmation is required.")
		return
	}
	if err := r.db.Model(&models.User{}).Where("id = ?", c.Param("id")).Update("status", models.IdentityActive).Error; err != nil {
		fail(c, err); return
	}
	c.JSON(http.StatusOK, gin.H{"status": "active"})
}

func (r *router) adminRetryAnchor(c *gin.Context) {
	var body struct { Password string `json:"password"` }
	if !httpx.BindJSON(c, &body) { return }
	if !r.auth.CheckPassword(c.GetString("user_id"), body.Password) {
		httpx.Error(c, http.StatusForbidden, "step_up_failed", "Recent password confirmation is required.")
		return
	}
	if err := r.db.Model(&models.AnchorAttempt{}).Where("id = ?", c.Param("id")).Update("status", "pending").Error; err != nil {
		fail(c, err); return
	}
	c.JSON(http.StatusOK, gin.H{"status": "pending"})
}

func (r *router) adminGetConfig(c *gin.Context) {
	var items []models.SystemConfig
	if err := r.db.Order("key asc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminPatchConfig(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
		Value    string `json:"value"`
	}
	if !httpx.BindJSON(c, &body) { return }
	if !r.auth.CheckPassword(c.GetString("user_id"), body.Password) {
		httpx.Error(c, http.StatusForbidden, "step_up_failed", "Recent password confirmation is required.")
		return
	}
	if err := r.db.Model(&models.SystemConfig{}).Where("key = ?", c.Param("key")).Update("value", body.Value).Error; err != nil {
		fail(c, err); return
	}
	c.JSON(http.StatusOK, gin.H{"status": "updated"})
}

func (r *router) adminAlerts(c *gin.Context) {
	var items []models.Alert
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminPatchAlert(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
		Status   string `json:"status"`
	}
	if !httpx.BindJSON(c, &body) { return }
	if !r.auth.CheckPassword(c.GetString("user_id"), body.Password) {
		httpx.Error(c, http.StatusForbidden, "step_up_failed", "Recent password confirmation is required.")
		return
	}
	if err := r.db.Model(&models.Alert{}).Where("id = ?", c.Param("id")).Update("status", body.Status).Error; err != nil {
		fail(c, err); return
	}
	c.JSON(http.StatusOK, gin.H{"status": body.Status})
}

func (r *router) adminCreateExport(c *gin.Context) {
	var body struct {
		Password string `json:"password"`
		Kind     string `json:"kind"`
	}
	if !httpx.BindJSON(c, &body) { return }
	if !r.auth.CheckPassword(c.GetString("user_id"), body.Password) {
		httpx.Error(c, http.StatusForbidden, "step_up_failed", "Recent password confirmation is required.")
		return
	}
	job := models.ExportJob{ID: uuid.NewString(), Requester: c.GetString("user_id"), Kind: body.Kind, Status: "requested", CreatedAt: time.Now().UTC()}
	if err := r.db.Create(&job).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusCreated, job)
}

func (r *router) adminListExports(c *gin.Context) {
	var items []models.ExportJob
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminDisputes(c *gin.Context) {
	var items []models.Dispute
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminCorrections(c *gin.Context) {
	var items []models.Correction
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminProvenanceEvents(c *gin.Context) {
	var items []models.ProvenanceEvent
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminCreditBatches(c *gin.Context) {
	var items []models.CreditBatch
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminListings(c *gin.Context) {
	var items []models.MarketplaceListing
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminPurchases(c *gin.Context) {
	var items []models.Purchase
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminRetirements(c *gin.Context) {
	var items []models.Retirement
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminFinancialImports(c *gin.Context) {
	var items []models.FinancialImport
	if err := r.db.Order("occurred_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminSatellitePasses(c *gin.Context) {
	var items []models.SatellitePass
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminSyncActions(c *gin.Context) {
	var items []models.SyncAction
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminMethodologies(c *gin.Context) {
	var items []models.Methodology
	if err := r.db.Order("created_at desc").Find(&items).Error; err != nil { fail(c, err); return }
	c.JSON(http.StatusOK, gin.H{"items": items})
}

func (r *router) adminSearch(c *gin.Context) {
	q := c.Query("q")
	result := gin.H{"users": []models.User{}, "projects": []models.ConservationProject{}, "activities": []models.ConservationActivity{}, "proofs": []models.ProofDocument{}}
	if q != "" {
		var users []models.User
		r.db.Where("email ILIKE ? OR display_name ILIKE ?", "%"+q+"%", "%"+q+"%").Find(&users)
		result["users"] = users
		var projects []models.ConservationProject
		r.db.Where("name ILIKE ?", "%"+q+"%").Find(&projects)
		result["projects"] = projects
		var activities []models.ConservationActivity
		r.db.Where("id::text ILIKE ?", "%"+q+"%").Limit(50).Find(&activities)
		result["activities"] = activities
		var proofs []models.ProofDocument
		r.db.Where("id::text ILIKE ?", "%"+q+"%").Limit(50).Find(&proofs)
		result["proofs"] = proofs
	}
	c.JSON(http.StatusOK, result)
}

func (r *router) requireAdmin(c *gin.Context) {
	if c.GetString("user_role") != models.RoleAdmin {
		httpx.Error(c, http.StatusForbidden, "forbidden", "Platform administrator access is required.")
		c.Abort()
		return
	}
	c.Next()
}

