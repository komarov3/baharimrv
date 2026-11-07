package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment         string
	Port                string
	DatabaseURL         string
	CORSOrigins         []string
	PublicBaseURL       string
	SessionTTL          time.Duration
	KMSKEK              []byte
	ChainMode           string
	IOTANetwork         string
	IOTARPCURL          string
	IOTAExecutorURL     string
	IOTAExecutorAPIKey  string
	IOTAPackageID       string
	SatelliteMode       string
	RegistryMode        string
	PaymentMode         string
	EvidenceRoot        string
	IOTAExplorerURL     string
	// Gas station: monitors sponsor wallet and auto-faucets when low
	IOTASponsorAddress  string
	IOTAGasThreshold    uint64
	GasExecutorURL      string
	OIDCIssuer          string
	OIDCSigningKeyPEM   string
	OIDCSigningKeyID    string
	FederationClients   map[string]string
	AuthRatePerMinute   int
	PublicRatePerMinute int
	MaxRequestBytes     int64
	SeedDemoData        bool
	DemoSeedPassword    string

	// Jahazi cross-platform integration
	JahaziEnabled       bool
	JahaziWebhookURL    string
	JahaziWebhookSecret string

	// BahariLink hub for cross-platform identity caching.
	BahariLinkHubURL string
	BahariLinkAPIKey string
}

func Load() (Config, error) {
	environment := value("APP_ENV", "development")
	chainMode := value("CHAIN_MODE", "sandbox")
	if environment == "production" && chainMode != "iota" {
		return Config{}, fmt.Errorf("production requires CHAIN_MODE=iota")
	}
	if environment == "production" && boolean("SEED_DEMO_DATA", false) {
		return Config{}, fmt.Errorf("demo seeding is forbidden in production")
	}
	databaseURL := value("DATABASE_URL", "file:baharimrv-local.db?_busy_timeout=5000&_journal_mode=WAL")
	if environment == "production" {
		if !strings.HasPrefix(databaseURL, "postgres://") && !strings.HasPrefix(databaseURL, "postgresql://") {
			return Config{}, fmt.Errorf("production DATABASE_URL must use PostgreSQL")
		}
		if strings.Contains(databaseURL, "sslmode=disable") {
			return Config{}, fmt.Errorf("production PostgreSQL SSL cannot be disabled")
		}
	}
	kekText := os.Getenv("KMS_KEK")
	var kek []byte
	var err error
	if kekText != "" {
		kek, err = base64.StdEncoding.DecodeString(kekText)
		if err != nil || len(kek) != 32 {
			return Config{}, fmt.Errorf("KMS_KEK must be a base64-encoded 32-byte key-encryption key")
		}
	} else if environment == "production" {
		return Config{}, fmt.Errorf("KMS_KEK is required in production")
	} else {
		kek = make([]byte, 32)
		if _, err = rand.Read(kek); err != nil {
			return Config{}, fmt.Errorf("generate development KEK: %w", err)
		}
	}
	origins := split(value("CORS_ORIGINS", "http://localhost:3000"))
	for _, origin := range origins {
		if origin == "*" {
			return Config{}, fmt.Errorf("wildcard CORS origins are forbidden")
		}
	}
	publicURL := value("PUBLIC_BASE_URL", "http://localhost:8080")
	if environment == "production" {
		parsed, parseErr := url.Parse(publicURL)
		if parseErr != nil || parsed.Scheme != "https" {
			return Config{}, fmt.Errorf("production PUBLIC_BASE_URL must be HTTPS")
		}
		if len(origins) == 0 {
			return Config{}, fmt.Errorf("production CORS_ORIGINS is required")
		}
		if os.Getenv("OIDC_SIGNING_PRIVATE_KEY") == "" || os.Getenv("OIDC_SIGNING_KID") == "" {
			return Config{}, fmt.Errorf("production OIDC signing key and kid are required")
		}
	}
	if boolean("SEED_DEMO_DATA", false) && os.Getenv("DEMO_SEED_PASSWORD") == "" {
		return Config{}, fmt.Errorf("DEMO_SEED_PASSWORD is required when demo seeding is enabled")
	}
	return Config{
		Environment:         environment,
		Port:                value("PORT", "8080"),
		DatabaseURL:         databaseURL,
		CORSOrigins:         origins,
		PublicBaseURL:       strings.TrimRight(publicURL, "/"),
		SessionTTL:          duration("SESSION_TTL", time.Hour),
		KMSKEK:              kek,
		ChainMode:           chainMode,
		IOTANetwork:         value("IOTA_NETWORK", "testnet"),
		IOTARPCURL:          value("IOTA_RPC_URL", "https://api.testnet.iota.cafe"),
		IOTAExecutorURL:     os.Getenv("IOTA_EXECUTOR_URL"),
		IOTAExecutorAPIKey:  os.Getenv("IOTA_EXECUTOR_API_KEY"),
		IOTAPackageID:       os.Getenv("IOTA_PACKAGE_ID"),
		SatelliteMode:       value("SATELLITE_MODE", "sandbox"),
		RegistryMode:        value("REGISTRY_MODE", "sandbox"),
		PaymentMode:         value("PAYMENT_MODE", "sandbox"),
		EvidenceRoot:        value("EVIDENCE_ROOT", "data/evidence"),
		IOTAExplorerURL:     os.Getenv("IOTA_EXPLORER_URL"),
		IOTASponsorAddress:  os.Getenv("IOTA_SPONSOR_ADDRESS"),
		IOTAGasThreshold:    uint64Value("IOTA_GAS_THRESHOLD", 100000000),
		GasExecutorURL:      os.Getenv("GAS_EXECUTOR_URL"),
		OIDCIssuer:          value("OIDC_ISSUER", strings.TrimRight(publicURL, "/")),
		OIDCSigningKeyPEM:   os.Getenv("OIDC_SIGNING_PRIVATE_KEY"),
		OIDCSigningKeyID:    value("OIDC_SIGNING_KID", "baharimrv-local-development"),
		FederationClients:   federationClients(os.Getenv("FEDERATION_CLIENTS")),
		AuthRatePerMinute:   integer("AUTH_RATE_PER_MINUTE", 10),
		PublicRatePerMinute: integer("PUBLIC_RATE_PER_MINUTE", 30),
		MaxRequestBytes:     int64(integer("MAX_REQUEST_BYTES", 2*1024*1024)),
		SeedDemoData:        boolean("SEED_DEMO_DATA", false),
		DemoSeedPassword:    os.Getenv("DEMO_SEED_PASSWORD"),
		JahaziEnabled:       boolean("JAHAZI_ENABLED", false),
		JahaziWebhookURL:    value("JAHAZI_WEBHOOK_URL", ""),
		JahaziWebhookSecret: os.Getenv("JAHAZI_WEBHOOK_SECRET"),
		BahariLinkHubURL:    os.Getenv("BAHARILINK_HUB_URL"),
		BahariLinkAPIKey:    os.Getenv("BAHARILINK_API_KEY"),
	}, nil
}

func value(key, fallback string) string {
	if current := os.Getenv(key); current != "" {
		return current
	}
	return fallback
}

func boolean(key string, fallback bool) bool {
	current := os.Getenv(key)
	if current == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(current)
	return err == nil && parsed
}

func integer(key string, fallback int) int {
	current, err := strconv.Atoi(os.Getenv(key))
	if err != nil || current <= 0 {
		return fallback
	}
	return current
}

func duration(key string, fallback time.Duration) time.Duration {
	current, err := time.ParseDuration(os.Getenv(key))
	if err != nil || current <= 0 {
		return fallback
	}
	return current
}

func split(current string) []string {
	var values []string
	for _, item := range strings.Split(current, ",") {
		if item = strings.TrimSpace(item); item != "" {
			values = append(values, item)
		}
	}
	return values
}

func federationClients(current string) map[string]string {
	clients := make(map[string]string)
	for _, item := range split(current) {
		parts := strings.SplitN(item, "=", 2)
		if len(parts) == 2 && strings.HasPrefix(parts[0], "https://") && parts[1] != "" {
			clients[strings.TrimRight(parts[0], "/")] = parts[1]
		}
	}
	return clients
}

func uint64Value(key string, fallback uint64) uint64 {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.ParseUint(v, 10, 64)
	if err != nil {
		return fallback
	}
	return n
}
