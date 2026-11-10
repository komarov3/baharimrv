// Package app is the single composition root for the BahariMRV API. The server
// binary and the behavioural tests build the same graph, so a test can never
// pass against wiring the production binary does not use.
package app

import (
	"errors"
	"net/http"
	"log/slog"
	"time"

	"github.com/baharilink/baharimrv/internal/api"
	"github.com/baharilink/baharimrv/internal/auth"
	"github.com/baharilink/baharimrv/internal/chain"
	"github.com/baharilink/baharimrv/internal/config"
	"github.com/baharilink/baharimrv/internal/database"
	"github.com/baharilink/baharimrv/internal/evidence"
	"github.com/baharilink/baharimrv/internal/identity"
	"github.com/baharilink/baharimrv/internal/jahazi"
	"github.com/baharilink/baharimrv/internal/market"
	"github.com/baharilink/baharimrv/internal/mrv"
	"github.com/baharilink/baharimrv/internal/notifications"
	"github.com/baharilink/baharimrv/internal/proof"
	"github.com/baharilink/baharimrv/internal/provenance"
	"github.com/baharilink/baharimrv/internal/satellite"
	"github.com/baharilink/baharimrv/internal/seed"
	"github.com/baharilink/baharimrv/internal/verification"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// App is the assembled API plus the pieces a caller needs to drive it: the shared
// database handle and the evidence content store.
type App struct {
	Handler  http.Handler
	DB       *gorm.DB
	Evidence *evidence.Store
}

// Build wires every adapter, service and route from cfg. Chain, satellite,
// registry and payment adapters are chosen by mode; a mode that asks for a live
// integration without its configuration fails instead of downgrading silently.
func Build(cfg config.Config) (*App, error) {
	gin.SetMode(gin.ReleaseMode)
	db, err := database.Open(cfg.DatabaseURL, cfg.Environment == "production")
	if err != nil {
		return nil, err
	}
	adapter, err := ChainAdapter(cfg)
	if err != nil {
		return nil, err
	}
	store, err := evidence.NewStore(cfg.EvidenceRoot, api.EvidenceFileBytes)
	if err != nil {
		return nil, err
	}
	custody, err := identity.NewEnvelopeCustody(cfg.KMSKEK)
	if err != nil {
		return nil, err
	}

	identities := identity.NewServiceWithHub(db, adapter, custody, cfg.BahariLinkHubURL, cfg.BahariLinkAPIKey)

	// Gas station: monitors sponsor wallet balance and auto-faucets when low.
	// Disabled in sandbox mode (no sponsor address configured).
	if cfg.ChainMode == "iota" && cfg.IOTASponsorAddress != "" {
		gs := chain.NewGasStation(cfg.IOTARPCURL, cfg.GasExecutorURL)
		go gs.PeriodicCheck(cfg.IOTASponsorAddress)
	}

	anchors := provenance.NewService(db, adapter, cfg.IOTAExplorerURL)
	imagery := satellite.NewService(db, satellite.Options{
		Mode: cfg.SatelliteMode, Provider: cfg.SatelliteProvider,
		WorkerURL: cfg.S2WorkerURL, WorkerToken: cfg.S2WorkerToken,
	})
	projects := mrv.NewService(db, identities, anchors, imagery, store)

	sessions := auth.NewService(db, identities, cfg.SessionTTL)
	signer, err := auth.NewSigner(cfg.OIDCSigningKeyPEM, cfg.OIDCSigningKeyID)
	if err != nil {
		return nil, err
	}
	oidcService := auth.NewOIDCService(db, signer, cfg.OIDCIssuer)
	stepUp := auth.NewStepUp(db, sessions)
	federation := auth.NewFederationService(db, auth.NewRemoteValidator(cfg.FederationClients), identities, sessions)
	verifications := verification.NewService(db, identities, anchors, projects, stepUp)
	marketplace := market.NewService(db, projects, anchors, stepUp, cfg.RegistryMode, cfg.PaymentMode)
	proofs := proof.NewService(db, projects, identities, anchors, imagery, store, cfg.RegistryMode, cfg.PublicBaseURL)
	jahaziService := jahazi.NewService(db, cfg.JahaziWebhookURL, cfg.JahaziWebhookSecret, cfg.IOTAExplorerURL)
	jahaziChain := jahazi.NewChainOps(db, adapter)
	notifier := notifications.NewService(db)
	jahaziService.SetNotifier(notifier)
	projects.SetRestorationMatcher(jahazi.NewMatcherAdapter(jahaziService))

	if cfg.SeedDemoData {
		// Run seed in the background so the HTTP server starts immediately
		// and passes Fly.io health checks. Seed provisions IOTA identities
		// which can take 10–30 s on testnet.
		go func() {
			if err := seed.Run(db, identities, projects, oidcService, cfg.DemoSeedPassword, time.Now().UTC()); err != nil {
				slog.Warn("demo seed failed", "error", err)
			}
		}()
	}

	return &App{
		Handler: api.New(api.Dependencies{
			Config: cfg, DB: db, Auth: sessions,
			AuthHandler:       auth.NewHandler(sessions, identities),
			OIDC:              auth.NewOIDCHandler(oidcService),
			Federation:        federation,
			FederationHandler: auth.NewFederationHandler(federation, cfg.FederationClients),
			StepUp:            stepUp, Identities: identities, Projects: projects,
			Anchors:      anchors,
			Satellite:    imagery,
			Verification: verification.NewHandler(verifications), VerificationService: verifications,
			Market: marketplace, Proof: proofs, Evidence: store,
			Jahazi:      jahaziService,
			JahaziChain: jahaziChain,
		}),
		DB: db, Evidence: store,
	}, nil
}

// ChainAdapter never silently downgrades a configured live network to the sandbox:
// CHAIN_MODE=iota requires the executor and package to be present.
func ChainAdapter(cfg config.Config) (chain.Adapter, error) {
	if cfg.ChainMode != "iota" {
		return chain.NewSandbox(), nil
	}
	if cfg.IOTAExecutorURL == "" || cfg.IOTAPackageID == "" {
		return nil, errors.New("CHAIN_MODE=iota requires IOTA_EXECUTOR_URL and IOTA_PACKAGE_ID")
	}
	if cfg.IOTANetwork != "testnet" {
		return nil, errors.New("this release anchors on IOTA Rebased testnet only")
	}
	return chain.NewIOTA(cfg.IOTARPCURL, cfg.IOTAExecutorURL, cfg.IOTAPackageID, cfg.IOTAExecutorAPIKey), nil
}
