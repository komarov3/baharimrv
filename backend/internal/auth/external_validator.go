package auth

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rsa"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type ExternalClaims struct {
	Issuer    string
	Subject   string
	Scope     string
	Name      string
	Email     string
	Phone     string
	ExpiresAt time.Time
}

type ExternalValidator interface {
	Validate(ctx context.Context, rawToken, expectedNonce, tokenUse string) (ExternalClaims, error)
	ExchangeCode(ctx context.Context, issuer, code, codeVerifier, redirectURI string) (string, error)
}

type RemoteValidator struct {
	clients    map[string]string
	httpClient *http.Client
	mu         sync.RWMutex
	keys       map[string]cachedKeys
	endpoints  map[string]cachedEndpoints
}

type cachedKeys struct {
	keys    map[string]any
	expires time.Time
}
type cachedEndpoints struct {
	token   string
	expires time.Time
}

func NewRemoteValidator(clients map[string]string) *RemoteValidator {
	return &RemoteValidator{
		clients: clients, httpClient: &http.Client{Timeout: 10 * time.Second},
		keys: make(map[string]cachedKeys), endpoints: make(map[string]cachedEndpoints),
	}
}

// ExchangeCode redeems an authorization code at the trusted issuer's token endpoint.
// PKCE is mandatory: BahariMRV is a public client and holds no secret.
func (v *RemoteValidator) ExchangeCode(ctx context.Context, issuer, code, codeVerifier, redirectURI string) (string, error) {
	issuer = strings.TrimRight(issuer, "/")
	clientID, trusted := v.clients[issuer]
	if !trusted {
		return "", fmt.Errorf("untrusted issuer")
	}
	tokenURL, err := v.tokenEndpoint(ctx, issuer)
	if err != nil {
		return "", err
	}
	form := url.Values{
		"grant_type": {"authorization_code"}, "code": {code},
		"code_verifier": {codeVerifier}, "redirect_uri": {redirectURI}, "client_id": {clientID},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	response, err := v.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("external token endpoint unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("external code exchange rejected")
	}
	var tokens struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 512*1024)).Decode(&tokens); err != nil {
		return "", fmt.Errorf("invalid external token response")
	}
	if tokens.IDToken == "" {
		return "", fmt.Errorf("external issuer returned no identity token")
	}
	return tokens.IDToken, nil
}

func (v *RemoteValidator) tokenEndpoint(ctx context.Context, issuer string) (string, error) {
	v.mu.RLock()
	cached := v.endpoints[issuer]
	v.mu.RUnlock()
	if cached.token != "" && time.Now().Before(cached.expires) {
		return cached.token, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return "", err
	}
	response, err := v.httpClient.Do(request)
	if err != nil {
		return "", fmt.Errorf("external discovery unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return "", fmt.Errorf("external discovery unavailable")
	}
	var discovery struct {
		Issuer        string `json:"issuer"`
		TokenEndpoint string `json:"token_endpoint"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1024*1024)).Decode(&discovery); err != nil ||
		strings.TrimRight(discovery.Issuer, "/") != issuer || !strings.HasPrefix(discovery.TokenEndpoint, "https://") {
		return "", fmt.Errorf("invalid external discovery")
	}
	v.mu.Lock()
	v.endpoints[issuer] = cachedEndpoints{token: discovery.TokenEndpoint, expires: time.Now().Add(5 * time.Minute)}
	v.mu.Unlock()
	return discovery.TokenEndpoint, nil
}

func (v *RemoteValidator) Validate(ctx context.Context, rawToken, expectedNonce, tokenUse string) (ExternalClaims, error) {
	unverified, _, err := jwt.NewParser().ParseUnverified(rawToken, jwt.MapClaims{})
	if err != nil {
		return ExternalClaims{}, fmt.Errorf("invalid external token")
	}
	claims, ok := unverified.Claims.(jwt.MapClaims)
	if !ok {
		return ExternalClaims{}, fmt.Errorf("invalid external claims")
	}
	issuer, _ := claims["iss"].(string)
	issuer = strings.TrimRight(issuer, "/")
	audience, trusted := v.clients[issuer]
	if !trusted {
		return ExternalClaims{}, fmt.Errorf("untrusted issuer")
	}
	parsed, err := jwt.Parse(rawToken, func(token *jwt.Token) (any, error) {
		kid, _ := token.Header["kid"].(string)
		if kid == "" || (token.Method.Alg() != "RS256" && token.Method.Alg() != "ES256") {
			return nil, fmt.Errorf("invalid external key")
		}
		return v.key(ctx, issuer, kid)
	}, jwt.WithAudience(audience), jwt.WithIssuer(issuer), jwt.WithExpirationRequired(), jwt.WithValidMethods([]string{"RS256", "ES256"}))
	if err != nil || !parsed.Valid {
		return ExternalClaims{}, fmt.Errorf("external token validation failed")
	}
	validated, _ := parsed.Claims.(jwt.MapClaims)
	if validated["token_use"] != tokenUse {
		return ExternalClaims{}, fmt.Errorf("incorrect token use")
	}
	if tokenUse == "id" {
		nonce, _ := validated["nonce"].(string)
		if expectedNonce == "" || subtle.ConstantTimeCompare([]byte(nonce), []byte(expectedNonce)) != 1 {
			return ExternalClaims{}, fmt.Errorf("nonce mismatch")
		}
	}
	subject, _ := validated["sub"].(string)
	if subject == "" {
		return ExternalClaims{}, fmt.Errorf("subject missing")
	}
	result := ExternalClaims{Issuer: issuer, Subject: subject}
	result.Scope, _ = validated["scope"].(string)
	result.Name, _ = validated["name"].(string)
	result.Email, _ = validated["email"].(string)
	result.Phone, _ = validated["phone_number"].(string)
	if exp, ok := validated["exp"].(float64); ok {
		result.ExpiresAt = time.Unix(int64(exp), 0).UTC()
	}
	return result, nil
}

func (v *RemoteValidator) key(ctx context.Context, issuer, kid string) (any, error) {
	v.mu.RLock()
	cached := v.keys[issuer]
	key := cached.keys[kid]
	v.mu.RUnlock()
	if key != nil && time.Now().Before(cached.expires) {
		return key, nil
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, issuer+"/.well-known/openid-configuration", nil)
	if err != nil {
		return nil, err
	}
	response, err := v.httpClient.Do(request)
	if err != nil {
		return nil, fmt.Errorf("external discovery unavailable")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("external discovery unavailable")
	}
	var discovery struct {
		Issuer  string `json:"issuer"`
		JWKSURI string `json:"jwks_uri"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1024*1024)).Decode(&discovery); err != nil || strings.TrimRight(discovery.Issuer, "/") != issuer || !strings.HasPrefix(discovery.JWKSURI, "https://") {
		return nil, fmt.Errorf("invalid external discovery")
	}
	jwksRequest, _ := http.NewRequestWithContext(ctx, http.MethodGet, discovery.JWKSURI, nil)
	jwksResponse, err := v.httpClient.Do(jwksRequest)
	if err != nil {
		return nil, fmt.Errorf("external JWKS unavailable")
	}
	defer jwksResponse.Body.Close()
	var document struct {
		Keys []map[string]any `json:"keys"`
	}
	if jwksResponse.StatusCode != http.StatusOK || json.NewDecoder(io.LimitReader(jwksResponse.Body, 2*1024*1024)).Decode(&document) != nil {
		return nil, fmt.Errorf("invalid external JWKS")
	}
	keys := make(map[string]any)
	for _, item := range document.Keys {
		keyID, _ := item["kid"].(string)
		parsed, err := parseJWK(item)
		if err == nil && keyID != "" {
			keys[keyID] = parsed
		}
	}
	v.mu.Lock()
	v.keys[issuer] = cachedKeys{keys: keys, expires: time.Now().Add(5 * time.Minute)}
	v.mu.Unlock()
	if keys[kid] == nil {
		return nil, fmt.Errorf("external signing key not found")
	}
	return keys[kid], nil
}

func parseJWK(item map[string]any) (any, error) {
	kty, _ := item["kty"].(string)
	switch kty {
	case "RSA":
		n, errN := base64.RawURLEncoding.DecodeString(fmt.Sprint(item["n"]))
		e, errE := base64.RawURLEncoding.DecodeString(fmt.Sprint(item["e"]))
		if errN != nil || errE != nil || len(n) < 256 {
			return nil, fmt.Errorf("invalid RSA JWK")
		}
		return &rsa.PublicKey{N: new(big.Int).SetBytes(n), E: int(new(big.Int).SetBytes(e).Int64())}, nil
	case "EC":
		if item["crv"] != "P-256" {
			return nil, fmt.Errorf("unsupported EC curve")
		}
		x, errX := base64.RawURLEncoding.DecodeString(fmt.Sprint(item["x"]))
		y, errY := base64.RawURLEncoding.DecodeString(fmt.Sprint(item["y"]))
		if errX != nil || errY != nil {
			return nil, fmt.Errorf("invalid EC JWK")
		}
		return &ecdsa.PublicKey{Curve: elliptic.P256(), X: new(big.Int).SetBytes(x), Y: new(big.Int).SetBytes(y)}, nil
	default:
		return nil, fmt.Errorf("unsupported JWK")
	}
}
