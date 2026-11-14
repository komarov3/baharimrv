// Package api assembles the HTTP surface: middleware, the OIDC provider endpoints,
// and every /api/v1 route the web and Android conservation clients call.
package api

import (
	"net/http"
	"time"

	"github.com/baharilink/baharimrv/internal/auth"
	"github.com/baharilink/baharimrv/internal/config"
	"github.com/baharilink/baharimrv/internal/evidence"
	"github.com/baharilink/baharimrv/internal/httpx"
	"github.com/baharilink/baharimrv/internal/identity"
	"github.com/baharilink/baharimrv/internal/jahazi"
	"github.com/baharilink/baharimrv/internal/market"
	"github.com/baharilink/baharimrv/internal/mrv"
	"github.com/baharilink/baharimrv/internal/proof"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/baharilink/baharimrv/internal/satellite"
	"github.com/baharilink/baharimrv/internal/verification"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// EvidenceFileBytes is the largest single evidence file the content store accepts.
const EvidenceFileBytes int64 = 25 * 1024 * 1024

// MaxEvidenceUploadBytes is the transport ceiling for one multipart upload. It sits
// just above EvidenceFileBytes so a valid file is rejected by the store's hash and
// size checks, not silently truncated by the HTTP layer.
const MaxEvidenceUploadBytes int64 = EvidenceFileBytes + 1024*1024

type Dependencies struct {
	Config              config.Config
	DB                  *gorm.DB
	Auth                *auth.Service
	AuthHandler         *auth.Handler
	OIDC                *auth.OIDCHandler
	Federation          *auth.FederationService
	FederationHandler   *auth.FederationHandler
	StepUp              *auth.StepUp
	Identities          *identity.Service
	Projects            *mrv.Service
	Anchors             *provenance.Service
	Satellite           *satellite.Service
	Verification        *verification.Handler
	VerificationService *verification.Service
	Market              *market.Service
	Proof               *proof.Service
	Evidence            *evidence.Store
	Jahazi              *jahazi.Service
	JahaziChain         *jahazi.ChainOps
}

type router struct {
	config       config.Config
	db           *gorm.DB
	auth         *auth.Service
	identities   *identity.Service
	links        *auth.FederationService
	projects     *mrv.Service
	anchors      *provenance.Service
	satellite    *satellite.Service
	market       *market.Service
	proof        *proof.Service
	evidence     *evidence.Store
	stepUp       *auth.StepUp
	verification *verification.Service
	jahazi       *jahazi.Service
	jahaziChain  *jahazi.ChainOps
}

func New(deps Dependencies) http.Handler {
	gin.SetMode(gin.ReleaseMode)
	engine := gin.New()
	production := deps.Config.Environment == "production"
	engine.Use(httpx.RequestID(), httpx.Recovery(), httpx.SecurityHeaders(production), httpx.CORS(deps.Config.CORSOrigins))

	r := &router{
		config: deps.Config, db: deps.DB, auth: deps.Auth, identities: deps.Identities, anchors: deps.Anchors,
		satellite: deps.Satellite, links: deps.Federation, projects: deps.Projects, market: deps.Market, proof: deps.Proof,
		evidence: deps.Evidence, stepUp: deps.StepUp, verification: deps.VerificationService,
		jahazi:      deps.Jahazi,
		jahaziChain: deps.JahaziChain,
	}

	authLimit := httpx.NewIPRateLimiter(deps.Config.AuthRatePerMinute).Middleware()
	publicLimit := httpx.NewIPRateLimiter(deps.Config.PublicRatePerMinute).Middleware()
	bodyLimit := httpx.BodyLimit(deps.Config.MaxRequestBytes)

	engine.GET("/healthz", r.health)
	engine.GET("/.well-known/openid-configuration", deps.OIDC.Discovery)
	engine.GET("/.well-known/jwks.json", deps.OIDC.JWKS)
	engine.GET("/jwks.json", deps.OIDC.JWKS)

	// BahariMRV acts as an OIDC provider for sibling BahariLink platforms. The
	// consent screen is the already-authenticated local session.
	engine.GET("/oauth/authorize", auth.RequireLocal(deps.Auth), deps.OIDC.Authorize)
	engine.POST("/oauth/token", authLimit, bodyLimit, deps.OIDC.Token)
	engine.GET("/oauth/userinfo", deps.OIDC.Userinfo)

	v1 := engine.Group("/api/v1")
	v1.GET("/health", r.health)

	v1.POST("/auth/register", authLimit, bodyLimit, deps.AuthHandler.Register)
	v1.POST("/auth/login", authLimit, bodyLimit, deps.AuthHandler.Login)
	v1.POST("/auth/refresh", authLimit, bodyLimit, deps.AuthHandler.Refresh)
	v1.POST("/auth/federation/exchange", authLimit, bodyLimit, deps.FederationHandler.Exchange)
	v1.POST("/auth/federation/:issuer/exchange", authLimit, bodyLimit, deps.FederationHandler.Exchange)

	v1.GET("/public/proofs/:id", publicLimit, r.publicProof)
	v1.POST("/public/proofs/:id/verify", publicLimit, bodyLimit, r.verifyProof)

	// Jahazi cross-platform integration — HMAC-verified endpoints (no session).
	// These receive RestorationClaims and contribution records from Jahazi.
	v1.POST("/restoration-claims", publicLimit, bodyLimit, r.receiveRestorationClaim)
	v1.GET("/restoration-claims/:claimId", publicLimit, r.getRestorationClaimStatus)
	v1.POST("/projects/:id/contributions", publicLimit, bodyLimit, r.receiveJahaziContribution)

	session := v1.Group("")
	session.Use(auth.RequireLocal(deps.Auth), auditWrites(deps.DB, "api_write"))
	{
		session.GET("/auth/me", deps.AuthHandler.Me)
		session.POST("/auth/logout", deps.AuthHandler.Logout)
		session.POST("/auth/step-up", bodyLimit, deps.AuthHandler.StepUp(deps.StepUp))
		session.POST("/oauth/grants/revoke", bodyLimit, deps.OIDC.RevokeGrant)

		session.POST("/auth/federation/link", bodyLimit, deps.FederationHandler.Link)
		session.POST("/auth/federation/:issuer/link", bodyLimit, deps.FederationHandler.Link)
		session.GET("/auth/links", r.linkedPlatforms)
		session.DELETE("/auth/links/:linkId", deps.FederationHandler.Unlink)

		session.GET("/users/me/identity", r.identityProfile)
		session.POST("/identity/provision/retry", bodyLimit, r.retryIdentity)
		session.POST("/users/me/identity/recover", bodyLimit, deps.AuthHandler.RecoverIdentity)
		session.POST("/users/me/identity/rotate", bodyLimit, deps.AuthHandler.RotateIdentity)

		session.GET("/projects", r.listJahaziProjects)
		session.GET("/projects/:id/activities", r.listActivities)
		session.GET("/projects/:id/weather", r.projectWeather)
		session.GET("/projects/:id/salinity", r.projectSalinity)
		session.GET("/monitors", r.listMonitors)
		session.PATCH("/projects/:id/plots/:plotId", bodyLimit, r.updatePlot)
		session.GET("/methodologies", r.methodologies)
		session.POST("/projects", bodyLimit, r.createProject)
		session.GET("/projects/:id/dashboard", r.dashboard)
		session.GET("/projects/:id/wallet", r.wallet)
		session.GET("/projects/:id/reconciliation", r.reconciliation)
		session.GET("/projects/:id/documents", r.projectDocuments)
		session.POST("/projects/:id/activities", bodyLimit, r.createActivity)
		session.POST("/projects/:id/plots", bodyLimit, r.addPlot)
		session.GET("/projects/:id/satellite/passes", r.projectSatellitePasses)
		session.GET("/projects/:id/satellite/latest", r.projectSatelliteLatest)
		session.GET("/projects/:id/satellite/change", r.projectSatelliteChange)
		session.POST("/projects/:id/satellite/refresh", bodyLimit, r.projectSatelliteRefresh)
		session.GET("/projects/:id/satellite/refreshes/:jobId", r.projectSatelliteRefreshStatus)
		session.GET("/projects/:id/satellite/overlay", r.projectSatelliteOverlay)
		session.GET("/projects/:id/satellite/tiles/:z/:x/:y", r.projectSatelliteTile)

		session.GET("/documents/:id", r.downloadDocument)
		session.POST("/sync/batch", bodyLimit, r.syncBatch)
		session.GET("/sync/delta", r.syncDelta)
		session.POST("/activities/:id/monitoring-records", bodyLimit, r.createMonitoringRecord)
		session.POST("/monitoring-records/:id/evidence", httpx.BodyLimit(MaxEvidenceUploadBytes), r.uploadEvidence)

		session.GET("/activities/:id/provenance", r.activityProvenance)
		session.GET("/activities/:id/detail", r.activityDetail)
		session.GET("/monitoring-records/evidence/:id", r.evidenceMedia)
		session.POST("/activities/:id/complete", bodyLimit, r.completeActivity)
		session.POST("/activities/:id/anchor/retry", bodyLimit, r.retryAnchor)
		session.POST("/activities/:id/dispute", bodyLimit, r.disputeActivity)
		session.POST("/activities/:id/submit-verification", bodyLimit, r.assignVerifier)
		session.GET("/anchors/:id/status", r.anchorStatus)

		// Jahazi integration — session-authenticated endpoints for PM/admin use.
		session.GET("/activities/:id/attribution-data", r.attributionData)
		session.POST("/activities/:id/mrv-link/generate", bodyLimit, r.generateMrvLink)

		// Jahazi chain callbacks — called by Jahazi backend to trigger on-chain ops.
		// These forward to the IOTA executor via the chain adapter.
		session.POST("/chain/contribution-entry", bodyLimit, r.chainContributionEntry)
		session.POST("/chain/issue-certificate", bodyLimit, r.chainIssueCertificate)

		session.GET("/verification-requests", deps.Verification.Queue)
		session.POST("/verification-requests/:id/decision", bodyLimit, deps.Verification.Decision)

		session.GET("/marketplace/listings", r.listings)
		session.POST("/marketplace/purchases", bodyLimit, r.purchase)
		session.POST("/credit-batches/:id/retire", bodyLimit, r.retire)

		session.GET("/achievements/:id/proofs", r.listProofs)
		session.POST("/achievements/:id/proofs", bodyLimit, r.generateProof)
		session.GET("/proofs/:id/download", r.downloadProof)

		session.GET("/admin/overview", r.requireAdmin, r.adminOverview)
		session.GET("/admin/satellite/health", r.requireAdmin, r.adminSatelliteHealth)
		session.GET("/admin/users", r.requireAdmin, r.adminUsers)
		session.GET("/admin/identities", r.requireAdmin, r.adminIdentities)
		session.GET("/admin/projects", r.requireAdmin, r.adminProjects)
		session.GET("/admin/activities", r.requireAdmin, r.adminActivities)
		session.GET("/admin/verification-requests", r.requireAdmin, r.adminVerificationRequests)
		session.GET("/admin/anchors", r.requireAdmin, r.adminAnchors)
		session.GET("/admin/proofs", r.requireAdmin, r.adminProofs)
		session.GET("/admin/audit-events", r.requireAdmin, r.adminAuditEvents)
		session.POST("/admin/users/:id/disable", r.requireAdmin, r.adminDisableUser)
		session.POST("/admin/users/:id/enable", r.requireAdmin, r.adminEnableUser)
		session.POST("/admin/anchors/:id/retry", r.requireAdmin, r.adminRetryAnchor)
		session.GET("/admin/config", r.requireAdmin, r.adminGetConfig)
		session.PATCH("/admin/config/:key", r.requireAdmin, r.adminPatchConfig)
		session.GET("/admin/alerts", r.requireAdmin, r.adminAlerts)
		session.PATCH("/admin/alerts/:id", r.requireAdmin, r.adminPatchAlert)
		session.GET("/admin/exports", r.requireAdmin, r.adminListExports)
		session.POST("/admin/exports", r.requireAdmin, r.adminCreateExport)
		session.GET("/admin/evidence", r.requireAdmin, r.adminEvidence)
		session.GET("/admin/data-quality", r.requireAdmin, r.adminDataQuality)
		session.GET("/admin/integrations", r.requireAdmin, r.adminIntegrations)
		session.GET("/admin/disputes", r.requireAdmin, r.adminDisputes)
		session.GET("/admin/corrections", r.requireAdmin, r.adminCorrections)
		session.GET("/admin/provenance-events", r.requireAdmin, r.adminProvenanceEvents)
		session.GET("/admin/credit-batches", r.requireAdmin, r.adminCreditBatches)
		session.GET("/admin/marketplace/listings", r.requireAdmin, r.adminListings)
		session.GET("/admin/marketplace/purchases", r.requireAdmin, r.adminPurchases)
		session.GET("/admin/retirements", r.requireAdmin, r.adminRetirements)
		session.GET("/admin/finance/imports", r.requireAdmin, r.adminFinancialImports)
		session.GET("/admin/satellite/passes", r.requireAdmin, r.adminSatellitePasses)
		session.GET("/admin/sync-actions", r.requireAdmin, r.adminSyncActions)
		session.GET("/admin/methodologies", r.requireAdmin, r.adminMethodologies)
		session.GET("/admin/search", r.requireAdmin, r.adminSearch)
	}

	engine.NoRoute(func(c *gin.Context) {
		httpx.Error(c, http.StatusNotFound, "not_found", "The requested endpoint does not exist.")
	})
	return engine
}

// auditWrites records mutating requests only; reads would bury the audit trail in
// dashboard polling.
func auditWrites(db *gorm.DB, action string) gin.HandlerFunc {
	audit := httpx.Audit(db, action)
	return func(c *gin.Context) {
		if c.Request.Method == http.MethodGet || c.Request.Method == http.MethodHead {
			c.Next()
			return
		}
		audit(c)
	}
}

func (r *router) health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": "ok",
		"modes": gin.H{
			"chain": r.config.ChainMode, "satellite": r.config.SatelliteMode,
			"satelliteProvider": r.satellite.ProviderName(),
			"registry":          r.config.RegistryMode, "payment": r.config.PaymentMode,
		},
		"serverTime": time.Now().UTC(),
	})
}
