// Package chain — Gas Station: manages sponsor wallet balance and auto-faucets.
//
// The gas station ensures the platform's sponsor wallet always has enough IOTA
// to sponsor conservation_chain transactions. It periodically checks the
// balance and requests from the faucet when it drops below a threshold.
//
// Pattern from EcoTrack's BlockchainService.EnsureGas / RequestTestnetFaucet.
// In BahariMRV this is a standalone service the executor calls before submitting
// transactions; the executor holds the wallet key.
package chain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

const (
	// Minimum balance below which we request from the faucet (in MIST = 10^-9 IOTA)
	MinBalanceMIST    = 50_000_000 // ~0.05 IOTA
	// Faucet request amount (the faucet decides; this is informational)
	FaucetCooldownSec  = 300        // 5 minutes between faucet requests
	// Balance check interval
	BalanceCheckMin    = 10         // check every 10 minutes
)

// GasStation monitors the sponsor wallet balance and auto-faucets when low.
type GasStation struct {
	rpcURL     string
	faucetURL  string
	logger     *slog.Logger
	lastFaucet time.Time
	httpClient *http.Client
}

// NewGasStation creates a gas station for the configured IOTA testnet.
func NewGasStation(rpcURL, faucetURL string) *GasStation {
	if faucetURL == "" {
		faucetURL = "https://faucet.testnet.iota.cafe/gas"
	}
	return &GasStation{
		rpcURL:     rpcURL,
		faucetURL:  faucetURL,
		logger:     slog.Default().With("component", "gas_station"),
		httpClient: &http.Client{Timeout: 30 * time.Second},
	}
}

// GetBalance queries the IOTA testnet for the current wallet balance.
func (g *GasStation) GetBalance(ctx context.Context, address string) (uint64, error) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  "iotax_getBalance",
		"params":  []any{address},
	}
	body, _ := json.Marshal(payload)
	resp, err := g.httpClient.Post(g.rpcURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("balance query: %w", err)
	}
	defer resp.Body.Close()
	var result map[string]any
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&result); err != nil {
		return 0, err
	}
	if res, ok := result["result"].(map[string]any); ok {
		if total, ok := res["totalBalance"].(string); ok {
			var balance uint64
			fmt.Sscanf(total, "%d", &balance)
			return balance, nil
		}
	}
	return 0, nil
}

// RequestFaucet requests testnet IOTA from the faucet for the sponsor address.
// Rate-limited to one request per FaucetCooldownSec.
func (g *GasStation) RequestFaucet(ctx context.Context, address string) error {
	if time.Since(g.lastFaucet) < time.Second*FaucetCooldownSec {
		g.logger.Debug("faucet cooldown active, skipping",
			"seconds_since_last", time.Since(g.lastFaucet).Seconds())
		return nil
	}

	payload := map[string]any{
		"FixedAmountRequest": map[string]any{
			"recipient": address,
		},
	}
	body, _ := json.Marshal(payload)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.faucetURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("faucet request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("faucet returned %d: %s", resp.StatusCode, string(b))
	}

	g.lastFaucet = time.Now()
	g.logger.Info("faucet request successful", "address", address)
	return nil
}

// EnsureGas checks the sponsor balance and faucets if below threshold.
// Called by the executor before submitting transactions.
func (g *GasStation) EnsureGas(ctx context.Context, address string) error {
	balance, err := g.GetBalance(ctx, address)
	if err != nil {
		g.logger.Warn("balance query failed, proceeding anyway", "error", err)
		return nil // non-fatal: transaction may still succeed
	}

	if balance >= MinBalanceMIST {
		g.logger.Debug("balance sufficient", "balance_mist", balance)
		return nil
	}

	g.logger.Info("balance low, requesting faucet",
		"balance_mist", balance, "threshold", MinBalanceMIST)
	return g.RequestFaucet(ctx, address)
}

// PeriodicCheck runs a background goroutine that checks balance every
// BalanceCheckMin minutes and faucets if needed. Call once at startup.
func (g *GasStation) PeriodicCheck(address string) {
	ticker := time.NewTicker(time.Minute * BalanceCheckMin)
	go func() {
		for range ticker.C {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			if err := g.EnsureGas(ctx, address); err != nil {
				g.logger.Warn("periodic gas check failed", "error", err)
			}
			cancel()
		}
	}()
}
