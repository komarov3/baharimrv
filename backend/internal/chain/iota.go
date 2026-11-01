package chain

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// IOTA targets IOTA Rebased testnet. Transaction construction and custodial
// signing are delegated to an explicitly configured execution service; this
// client never claims success from an HTTP submission alone.
type IOTA struct {
	rpcURL      string
	executorURL string
	packageID   string
	httpClient  *http.Client
}

func NewIOTA(rpcURL, executorURL, packageID string) *IOTA {
	return &IOTA{
		rpcURL:      strings.TrimRight(rpcURL, "/"),
		executorURL: strings.TrimRight(executorURL, "/"),
		packageID:   packageID,
		httpClient:  &http.Client{Timeout: 20 * time.Second},
	}
}

func (i *IOTA) Mode() string    { return "iota" }
func (i *IOTA) Network() string { return "testnet" }

func (i *IOTA) ProvisionIdentity(ctx context.Context, userID string, publicKey []byte) (IdentityResult, error) {
	if i.executorURL == "" || i.packageID == "" {
		return IdentityResult{}, fmt.Errorf("%w: identity executor or package is not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":       "provision_identity",
		"package_id":      i.packageID,
		"idempotency_key": userID,
		"public_key":      hex.EncodeToString(publicKey),
		"network":         "testnet",
	}
	var response struct {
		DID            string `json:"did"`
		DIDDocumentRef string `json:"did_document_reference"`
		Address        string `json:"address"`
		TransactionRef string `json:"transaction_reference"`
	}
	if err := i.execute(ctx, payload, &response); err != nil {
		return IdentityResult{}, err
	}
	if response.DID == "" || response.DIDDocumentRef == "" || response.Address == "" || response.TransactionRef == "" {
		return IdentityResult{}, fmt.Errorf("identity executor returned an incomplete result")
	}
	valid, err := i.ResolveIdentity(ctx, response.DIDDocumentRef)
	if err != nil || !valid {
		return IdentityResult{}, fmt.Errorf("identity document readback did not confirm provisioning")
	}
	return IdentityResult{response.DID, response.DIDDocumentRef, response.Address, response.TransactionRef, "testnet"}, nil
}

func (i *IOTA) Anchor(ctx context.Context, request AnchorRequest) (AnchorRecord, error) {
	if i.executorURL == "" || i.packageID == "" {
		return AnchorRecord{}, fmt.Errorf("%w: anchor executor or package is not configured", ErrUnavailable)
	}
	payload := map[string]any{
		"operation":          "anchor",
		"package_id":         i.packageID,
		"idempotency_key":    request.IdempotencyKey,
		"kind":               request.Kind,
		"commitment":         request.Commitment,
		"canonical_json":     request.CanonicalJSON,
		"issuer_did":         request.IssuerDID,
		"previous_object_id": request.PreviousObjectID,
		"network":            "testnet",
	}
	var response AnchorRecord
	if err := i.execute(ctx, payload, &response); err != nil {
		return AnchorRecord{}, err
	}
	if response.ObjectID == "" || response.TransactionRef == "" {
		return AnchorRecord{}, fmt.Errorf("anchor executor returned an incomplete result")
	}
	response.Network = "testnet"
	return response, nil
}

func (i *IOTA) ReadAnchor(ctx context.Context, objectID string) (AnchorRecord, error) {
	fields, err := i.objectFields(ctx, objectID)
	if err != nil {
		return AnchorRecord{}, err
	}
	record := AnchorRecord{
		ObjectID:       objectID,
		Commitment:     fieldString(fields, "commitment"),
		IssuerDID:      fieldString(fields, "issuer_did"),
		Status:         fieldString(fields, "status"),
		SupersededByID: fieldString(fields, "superseded_by_id"),
		Revoked:        fieldBool(fields, "revoked"),
		Network:        "testnet",
	}
	if record.Commitment == "" {
		if raw, ok := fields["commitment"].([]any); ok {
			bytesValue := make([]byte, 0, len(raw))
			for _, item := range raw {
				number, ok := item.(float64)
				if !ok || number < 0 || number > 255 {
					return AnchorRecord{}, fmt.Errorf("invalid chain commitment encoding")
				}
				bytesValue = append(bytesValue, byte(number))
			}
			record.Commitment = hex.EncodeToString(bytesValue)
		}
	}
	if record.Commitment == "" || record.IssuerDID == "" {
		return AnchorRecord{}, fmt.Errorf("chain object is not a BahariMRV anchor")
	}
	return record, nil
}

func (i *IOTA) ResolveIdentity(ctx context.Context, didDocumentRef string) (bool, error) {
	fields, err := i.objectFields(ctx, didDocumentRef)
	if err != nil {
		return false, err
	}
	if revoked := fieldBool(fields, "revoked"); revoked {
		return false, nil
	}
	return fieldString(fields, "id") != "" || fieldString(fields, "did") != "" || len(fields) > 0, nil
}

func (i *IOTA) execute(ctx context.Context, payload any, output any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, i.executorURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := i.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("execute testnet transaction: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1024))
		return fmt.Errorf("testnet executor returned HTTP %d", response.StatusCode)
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 2*1024*1024)).Decode(output); err != nil {
		return fmt.Errorf("decode testnet executor response: %w", err)
	}
	return nil
}

func (i *IOTA) objectFields(ctx context.Context, objectID string) (map[string]any, error) {
	if objectID == "" {
		return nil, ErrNotFound
	}
	payload := map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "iota_getObject",
		"params": []any{objectID, map[string]any{"showContent": true}},
	}
	body, _ := json.Marshal(payload)
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, i.rpcURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := i.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("%w: testnet readback failed", ErrUnavailable)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: testnet readback HTTP %d", ErrUnavailable, response.StatusCode)
	}
	var rpc struct {
		Result struct {
			Data struct {
				Content struct {
					Fields map[string]any `json:"fields"`
				} `json:"content"`
			} `json:"data"`
			Error *struct {
				Code int `json:"code"`
			} `json:"error"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 4*1024*1024)).Decode(&rpc); err != nil {
		return nil, fmt.Errorf("decode testnet readback: %w", err)
	}
	if rpc.Error != nil || rpc.Result.Error != nil || rpc.Result.Data.Content.Fields == nil {
		return nil, ErrNotFound
	}
	fields := rpc.Result.Data.Content.Fields
	if nested, ok := fields["fields"].(map[string]any); ok {
		return nested, nil
	}
	return fields, nil
}

func fieldString(fields map[string]any, key string) string {
	value, _ := fields[key].(string)
	return value
}

func fieldBool(fields map[string]any, key string) bool {
	value, _ := fields[key].(bool)
	return value
}
