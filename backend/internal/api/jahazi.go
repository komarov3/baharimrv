// Package api — Jahazi integration handlers.
//
// These endpoints handle cross-platform communication with the Jahazi tourism
// platform. HMAC-verified endpoints (restoration-claims, contributions) live
// outside the session-auth group; session-authenticated endpoints (attribution
// data, MRV link generation) live inside it.
package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/baharilink/baharimrv/internal/httpx"
	"github.com/baharilink/baharimrv/internal/jahazi"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/gin-gonic/gin"
)
// jahaziHandlers is embedded into the router struct via the Dependencies.jahazi field.
// The router methods below are the HTTP handlers for the Jahazi integration.

// ── HMAC-verified endpoints (no session auth) ──────────────────────────────

// receiveRestorationClaim handles POST /api/v1/restoration-claims from Jahazi.
// The request body is HMAC-signed with the shared JAHAZI_WEBHOOK_SECRET.
func (r *router) receiveRestorationClaim(c *gin.Context) {
	if r.jahazi == nil || !r.jahazi.Enabled() {
		httpx.Error(c, http.StatusServiceUnavailable, "jahazi_disabled", "Jahazi integration is not configured.")
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "read_error", "Failed to read request body.")
		return
	}

	// Verify HMAC signature from x-bahari-signature header.
	signature := c.GetHeader("x-bahari-signature")
	if !r.verifyHmac(body, signature) {
		httpx.Error(c, http.StatusUnauthorized, "invalid_signature", "HMAC signature verification failed.")
		return
	}

	var input jahazi.ClaimInput
	if err := json.Unmarshal(body, &input); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_json", "Invalid JSON payload.")
		return
	}

	req, err := r.jahazi.ReceiveClaim(input)
	if err != nil {
		switch err {
		case jahazi.ErrInvalidSignature:
			httpx.Error(c, http.StatusUnauthorized, "invalid_signature", err.Error())
		case jahazi.ErrDuplicateClaim:
			httpx.Error(c, http.StatusConflict, "duplicate_claim", err.Error())
		case jahazi.ErrProjectNotFound:
			httpx.Error(c, http.StatusNotFound, "project_not_found", err.Error())
		case jahazi.ErrNotJahaziEnabled:
			httpx.Error(c, http.StatusBadRequest, "not_jahazi_enabled", err.Error())
		default:
			httpx.Error(c, http.StatusBadRequest, "claim_error", err.Error())
		}
		return
	}

	// Anchor the restoration claim on-chain (non-blocking)
	if r.jahaziChain != nil {
		go func() {
			bgCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if _, err := r.jahaziChain.AnchorRestorationClaim(bgCtx, &req); err != nil {
				slog.Warn("AnchorRestorationClaim failed (non-critical)", "claimId", req.JahaziClaimID, "error", err)
			}
		}()
	}

	c.JSON(http.StatusOK, gin.H{
		"claimId":   req.JahaziClaimID,
		"status":    req.Status,
		"expiresAt": req.ExpiresAt.UTC().Format("2006-01-02T15:04:05Z"),
		"acceptedAt": req.CreatedAt.UTC().Format("2006-01-02T15:04:05Z"),
	})
}

// getRestorationClaimStatus handles GET /api/v1/restoration-claims/:claimId.
// Returns the current state of a RestorationRequest for Jahazi to poll.
func (r *router) getRestorationClaimStatus(c *gin.Context) {
	if r.jahazi == nil || !r.jahazi.Enabled() {
		httpx.Error(c, http.StatusServiceUnavailable, "jahazi_disabled", "Jahazi integration is not configured.")
		return
	}

	// Verify HMAC for authenticated polling.
	signature := c.GetHeader("x-bahari-signature")
	if !r.verifyHmac([]byte(c.Param("claimId")), signature) {
		httpx.Error(c, http.StatusUnauthorized, "invalid_signature", "HMAC signature verification failed.")
		return
	}

	req, err := r.jahazi.GetClaimStatus(c.Param("claimId"))
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "claim_not_found", err.Error())
		return
	}

	response := gin.H{
		"claimId":         req.JahaziClaimID,
		"status":          req.Status,
		"totalUnits":      req.TotalUnits,
		"fulfilledUnits":  req.FulfilledUnits,
		"species":         req.Species,
	}
	if req.MatchedActivityID != "" {
		response["matchedActivityId"] = req.MatchedActivityID
	}
	c.JSON(http.StatusOK, response)
}

// receiveJahaziContribution handles POST /api/v1/projects/:id/contributions.
// Records a Jahazi tourist contribution as a FinancialImport on BahariMRV.
func (r *router) receiveJahaziContribution(c *gin.Context) {
	if r.jahazi == nil || !r.jahazi.Enabled() {
		httpx.Error(c, http.StatusServiceUnavailable, "jahazi_disabled", "Jahazi integration is not configured.")
		return
	}

	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "read_error", "Failed to read request body.")
		return
	}

	signature := c.GetHeader("x-bahari-signature")
	if !r.verifyHmac(body, signature) {
		httpx.Error(c, http.StatusUnauthorized, "invalid_signature", "HMAC signature verification failed.")
		return
	}

	var input struct {
		JahaziContributionID string `json:"jahaziContributionId"`
		JahaziProjectID      string `json:"jahaziProjectId"`
		TouristDID           string `json:"touristId"`
		AmountUsd            string `json:"amountUsd"`
		SeedlingsEquivalent  int    `json:"seedlingsEquivalent"`
		ContributedAt        string `json:"contributedAt"`
		Nonce                string `json:"nonce"`
		Signature            string `json:"signature"`
	}
	if err := json.Unmarshal(body, &input); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_json", "Invalid JSON payload.")
		return
	}

	imp, err := r.jahazi.ReceiveContribution(toContributionInput(input))
	if err != nil {
		switch err {
		case jahazi.ErrInvalidSignature:
			httpx.Error(c, http.StatusUnauthorized, "invalid_signature", err.Error())
		case jahazi.ErrProjectNotFound:
			httpx.Error(c, http.StatusNotFound, "project_not_found", err.Error())
		default:
			httpx.Error(c, http.StatusBadRequest, "contribution_error", err.Error())
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"contributionId": imp.ID,
		"status":         "recorded",
	})
}

// ── Session-authenticated endpoints ────────────────────────────────────────

// attributionData handles GET /api/v1/activities/:id/attribution-data.
// Returns per-tree CO2e, IOTA anchor data, and proof ID for certificate generation.
func (r *router) attributionData(c *gin.Context) {
	if r.jahazi == nil || !r.jahazi.Enabled() {
		httpx.Error(c, http.StatusServiceUnavailable, "jahazi_disabled", "Jahazi integration is not configured.")
		return
	}

	data, err := r.jahazi.AttributionData(c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, data)
}

// generateMrvLink handles POST /api/v1/activities/:id/mrv-link/generate.
// Generates a signed MRV link payload for pushing to Jahazi.
func (r *router) generateMrvLink(c *gin.Context) {
	if r.jahazi == nil || !r.jahazi.Enabled() {
		httpx.Error(c, http.StatusServiceUnavailable, "jahazi_disabled", "Jahazi integration is not configured.")
		return
	}

	var body struct {
		JahaziProjectID string `json:"jahaziProjectId"`
		JahaziSlug      string `json:"jahaziSlug"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_json", "jahaziProjectId is required.")
		return
	}

	if body.JahaziProjectID == "" {
		httpx.Error(c, http.StatusBadRequest, "missing_field", "jahaziProjectId is required.")
		return
	}

	payload, err := r.jahazi.GenerateMrvLink(c.Param("id"), body.JahaziProjectID)
	if err != nil {
		fail(c, err)
		return
	}

	// Best-effort push to Jahazi (non-blocking on response).
	if body.JahaziSlug != "" {
		go func() {
			if err := r.jahazi.PushMrvLink(c.Request.Context(), payload, body.JahaziSlug); err != nil {
				// Logged inside PushMrvLink; don't block the response.
				_ = err
			}
		}()
	}

	c.JSON(http.StatusOK, payload)
}

// listJahaziProjects handles GET /api/v1/projects?jahazi_enabled=true.
// Returns projects visible to Jahazi for the conservation catalog. When the
// query param is absent, falls through to the normal listProjects handler.
func (r *router) listJahaziProjects(c *gin.Context) {
	if c.Query("jahazi_enabled") != "true" {
		r.listProjects(c)
		return
	}

	if r.jahazi == nil || !r.jahazi.Enabled() {
		httpx.Error(c, http.StatusServiceUnavailable, "jahazi_disabled", "Jahazi integration is not configured.")
		return
	}

	projects, err := r.jahazi.ListJahaziProjects()
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"projects": projects})
}

// ── HMAC helper ────────────────────────────────────────────────────────────

// verifyHmac checks an HMAC-SHA256 signature against the raw body using the
// shared secret. When the secret is not configured, verification fails closed.
func (r *router) verifyHmac(body []byte, signature string) bool {
	secret := r.config.JahaziWebhookSecret
	if secret == "" || signature == "" {
		return false
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(signature))
}

// ── Small helpers ──────────────────────────────────────────────────────────

// toContributionInput converts the HTTP body to the models.JahaziContribution struct.
func toContributionInput(input struct {
	JahaziContributionID string `json:"jahaziContributionId"`
	JahaziProjectID      string `json:"jahaziProjectId"`
	TouristDID           string `json:"touristId"`
	AmountUsd            string `json:"amountUsd"`
	SeedlingsEquivalent  int    `json:"seedlingsEquivalent"`
	ContributedAt        string `json:"contributedAt"`
	Nonce                string `json:"nonce"`
	Signature            string `json:"signature"`
}) models.JahaziContribution {
	return models.JahaziContribution{
		JahaziContributionID: input.JahaziContributionID,
		JahaziProjectID:      input.JahaziProjectID,
		TouristDID:           input.TouristDID,
		AmountUsd:            input.AmountUsd,
		SeedlingsEquivalent:  input.SeedlingsEquivalent,
		ContributedAt:        input.ContributedAt,
		Nonce:                input.Nonce,
		Signature:            input.Signature,
	}
}

// chainContributionEntry handles POST /api/v1/chain/contribution-entry.
// Called by Jahazi after certificate issuance to append a ContributionLedgerEntry
// to the ProjectImpactLedger on IOTA.
func (r *router) chainContributionEntry(c *gin.Context) {
	if r.jahaziChain == nil {
		httpx.Error(c, http.StatusServiceUnavailable, "chain_disabled", "Chain operations not configured.")
		return
	}
	var body struct {
		LedgerObjectID   string `json:"ledgerObjectId"`
		ClaimAnchorID    string `json:"claimAnchorId"`
		AmountCents      int64  `json:"amountCents"`
		ContributorCount int64  `json:"contributorCount"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_json", "Invalid payload.")
		return
	}
	ctx := c.Request.Context()
	res, err := r.jahazi.GetAdapter().AppendPlantingEntry(ctx, jahazi.PlantingEntryFromContribution(body.LedgerObjectID, body.ClaimAnchorID, body.AmountCents, body.ContributorCount))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

// chainIssueCertificate handles POST /api/v1/chain/issue-certificate.
// Called by Jahazi to issue a soul-bound ConservationCertificate NFT on IOTA.
func (r *router) chainIssueCertificate(c *gin.Context) {
	if r.jahaziChain == nil {
		httpx.Error(c, http.StatusServiceUnavailable, "chain_disabled", "Chain operations not configured.")
		return
	}
	// The executor handles the actual Move call; we forward the request.
	var body json.RawMessage
	if err := c.ShouldBindJSON(&body); err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_json", "Invalid payload.")
		return
	}
	// Forward to the IOTA executor via the jahazi service
	result, err := r.jahazi.ForwardCertificateToExecutor(c.Request.Context(), body)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, result)
}
