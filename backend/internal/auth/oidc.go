package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/baharilink/baharimrv/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var supportedScopes = map[string]bool{
	"openid": true, "profile": true, "email": true, "phone": true,
	"conservation:read": true, "conservation:financial:read": true,
}
var pkcePattern = regexp.MustCompile(`^[A-Za-z0-9_-]{43,128}$`)

type OIDCService struct {
	db     *gorm.DB
	signer *Signer
	issuer string
	now    func() time.Time
}

type AuthorizeParams struct {
	ClientID            string
	RedirectURI         string
	ResponseType        string
	Scope               string
	State               string
	Nonce               string
	CodeChallenge       string
	CodeChallengeMethod string
}

type TokenResult struct {
	AccessToken string `json:"access_token"`
	TokenType   string `json:"token_type"`
	ExpiresIn   int64  `json:"expires_in"`
	IDToken     string `json:"id_token"`
	Scope       string `json:"scope"`
}

func NewOIDCService(db *gorm.DB, signer *Signer, issuer string) *OIDCService {
	return &OIDCService{db: db, signer: signer, issuer: strings.TrimRight(issuer, "/"), now: time.Now}
}

func (s *OIDCService) RegisterClient(clientID, secret string, redirects []string, scopes string) (models.OIDCClient, error) {
	if !regexp.MustCompile(`^[A-Za-z0-9._-]{3,128}$`).MatchString(clientID) || len(redirects) == 0 {
		return models.OIDCClient{}, fmt.Errorf("invalid client registration")
	}
	for _, redirect := range redirects {
		if !ValidRedirectURI(redirect) {
			return models.OIDCClient{}, fmt.Errorf("invalid redirect URI")
		}
	}
	validatedScopes, err := validateScopes(scopes, "openid profile email phone conservation:read conservation:financial:read", false)
	if err != nil {
		return models.OIDCClient{}, err
	}
	encodedRedirects, _ := json.Marshal(redirects)
	var secretHash string
	if secret != "" {
		if len(secret) < 32 {
			return models.OIDCClient{}, fmt.Errorf("client secret is too short")
		}
		hash, err := bcrypt.GenerateFromPassword([]byte(secret), 12)
		if err != nil {
			return models.OIDCClient{}, err
		}
		secretHash = string(hash)
	}
	now := s.now().UTC()
	client := models.OIDCClient{ID: clientID, SecretHash: secretHash, RedirectURIs: string(encodedRedirects), AllowedScopes: validatedScopes, CreatedAt: now, UpdatedAt: now}
	if err := s.db.Create(&client).Error; err != nil {
		return models.OIDCClient{}, fmt.Errorf("client already exists or is invalid")
	}
	return client, nil
}

func (s *OIDCService) Discovery() map[string]any {
	return map[string]any{
		"issuer":                                s.issuer,
		"authorization_endpoint":                s.issuer + "/oauth/authorize",
		"token_endpoint":                        s.issuer + "/oauth/token",
		"userinfo_endpoint":                     s.issuer + "/oauth/userinfo",
		"jwks_uri":                              s.issuer + "/jwks.json",
		"response_types_supported":              []string{"code"},
		"grant_types_supported":                 []string{"authorization_code"},
		"code_challenge_methods_supported":      []string{"S256"},
		"scopes_supported":                      []string{"openid", "profile", "email", "phone", "conservation:read", "conservation:financial:read"},
		"claims_supported":                      []string{"sub", "name", "email", "phone_number", "nonce", "token_use"},
		"id_token_signing_alg_values_supported": []string{s.signer.method.Alg()},
	}
}

func (s *OIDCService) Authorize(userID string, params AuthorizeParams) (string, error) {
	var client models.OIDCClient
	if err := s.db.Where("id = ? AND disabled = false", params.ClientID).First(&client).Error; err != nil {
		return "", fmt.Errorf("unknown or disabled client")
	}
	if !registeredRedirect(client, params.RedirectURI) {
		return "", fmt.Errorf("redirect URI does not exactly match a registered URI")
	}
	if params.ResponseType != "code" || params.State == "" || params.Nonce == "" || params.CodeChallengeMethod != "S256" || !pkcePattern.MatchString(params.CodeChallenge) {
		return "", fmt.Errorf("authorization code, state, nonce, and PKCE S256 are required")
	}
	scopes, err := validateScopes(params.Scope, client.AllowedScopes, true)
	if err != nil {
		return "", err
	}
	var user models.User
	if err := s.db.Where("id = ? AND status = ?", userID, models.IdentityActive).First(&user).Error; err != nil {
		return "", fmt.Errorf("active local account required")
	}
	now := s.now().UTC()
	rawCode := make([]byte, 32)
	if _, err := rand.Read(rawCode); err != nil {
		return "", err
	}
	code := base64.RawURLEncoding.EncodeToString(rawCode)
	grant := models.OAuthGrant{ID: uuid.NewString(), UserID: userID, ClientID: client.ID, Scope: scopes, CreatedAt: now}
	record := models.AuthorizationCode{
		ID: uuid.NewString(), CodeHash: tokenHash(code), UserID: userID, ClientID: client.ID,
		GrantID: grant.ID, RedirectURI: params.RedirectURI, Scope: scopes, Nonce: params.Nonce,
		CodeChallenge: params.CodeChallenge, ExpiresAt: now.Add(2 * time.Minute), CreatedAt: now,
	}
	if err := s.db.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&grant).Error; err != nil {
			return err
		}
		return tx.Create(&record).Error
	}); err != nil {
		return "", err
	}
	target, _ := url.Parse(params.RedirectURI)
	query := target.Query()
	query.Set("code", code)
	query.Set("state", params.State)
	target.RawQuery = query.Encode()
	return target.String(), nil
}

func (s *OIDCService) Token(clientID, clientSecret, grantType, code, verifier, redirectURI string) (TokenResult, error) {
	if grantType != "authorization_code" || code == "" || !pkcePattern.MatchString(verifier) || redirectURI == "" {
		return TokenResult{}, fmt.Errorf("invalid token request")
	}
	var result TokenResult
	err := s.db.Transaction(func(tx *gorm.DB) error {
		var record models.AuthorizationCode
		if err := tx.Where("code_hash = ?", tokenHash(code)).First(&record).Error; err != nil {
			return errors.New("invalid authorization code")
		}
		if record.UsedAt != nil || record.ClientID != clientID || record.RedirectURI != redirectURI || !s.now().Before(record.ExpiresAt) {
			return errors.New("invalid, expired, or replayed authorization code")
		}
		var client models.OIDCClient
		if err := tx.Where("id = ? AND disabled = false", clientID).First(&client).Error; err != nil {
			return errors.New("disabled or unknown client")
		}
		if client.SecretHash != "" && bcrypt.CompareHashAndPassword([]byte(client.SecretHash), []byte(clientSecret)) != nil {
			return errors.New("client authentication failed")
		}
		if !registeredRedirect(client, redirectURI) {
			return errors.New("redirect mismatch")
		}
		challenge := sha256.Sum256([]byte(verifier))
		if base64.RawURLEncoding.EncodeToString(challenge[:]) != record.CodeChallenge {
			return errors.New("PKCE verification failed")
		}
		var grant models.OAuthGrant
		if err := tx.Where("id = ? AND revoked_at IS NULL", record.GrantID).First(&grant).Error; err != nil {
			return errors.New("grant revoked")
		}
		now := s.now().UTC()
		update := tx.Model(&models.AuthorizationCode{}).Where("id = ? AND used_at IS NULL", record.ID).Update("used_at", now)
		if update.Error != nil || update.RowsAffected != 1 {
			return errors.New("authorization code replay")
		}
		accessID := uuid.NewString()
		idToken, err := s.signer.Sign(map[string]any{
			"iss": s.issuer, "sub": record.UserID, "aud": clientID,
			"iat": now.Unix(), "exp": now.Add(10 * time.Minute).Unix(),
			"nonce": record.Nonce, "token_use": "id", "jti": uuid.NewString(),
		})
		if err != nil {
			return err
		}
		accessToken, err := s.signer.Sign(map[string]any{
			"iss": s.issuer, "sub": record.UserID, "aud": clientID,
			"iat": now.Unix(), "exp": now.Add(10 * time.Minute).Unix(),
			"token_use": "access", "scope": record.Scope, "grant_id": grant.ID, "jti": accessID,
		})
		if err != nil {
			return err
		}
		if err := tx.Create(&models.AccessToken{
			ID: accessID, TokenHash: tokenHash(accessToken), GrantID: grant.ID, UserID: record.UserID,
			ClientID: clientID, Scope: record.Scope, ExpiresAt: now.Add(10 * time.Minute), CreatedAt: now,
		}).Error; err != nil {
			return err
		}
		result = TokenResult{AccessToken: accessToken, TokenType: "Bearer", ExpiresIn: 600, IDToken: idToken, Scope: record.Scope}
		return nil
	})
	return result, err
}

func (s *OIDCService) Userinfo(accessToken string) (map[string]any, error) {
	claims, err := s.signer.Parse(accessToken)
	if err != nil || claims["token_use"] != "access" || claims["iss"] != s.issuer {
		return nil, fmt.Errorf("a valid access token is required")
	}
	subject, _ := claims["sub"].(string)
	var token models.AccessToken
	if err := s.db.Where("token_hash = ? AND user_id = ? AND expires_at > ?", tokenHash(accessToken), subject, s.now().UTC()).First(&token).Error; err != nil {
		return nil, fmt.Errorf("access token is not active")
	}
	var grant models.OAuthGrant
	if err := s.db.Where("id = ? AND revoked_at IS NULL", token.GrantID).First(&grant).Error; err != nil {
		return nil, fmt.Errorf("grant revoked")
	}
	var user models.User
	if err := s.db.Where("id = ? AND status = ?", subject, models.IdentityActive).First(&user).Error; err != nil {
		return nil, fmt.Errorf("account unavailable")
	}
	result := map[string]any{"sub": user.ID}
	for _, scope := range strings.Fields(grant.Scope) {
		switch scope {
		case "profile":
			result["name"] = user.DisplayName
		case "email":
			result["email"] = user.Email
			result["email_verified"] = true
		case "phone":
			if user.Phone != "" {
				result["phone_number"] = user.Phone
			}
		}
	}
	return result, nil
}

func (s *OIDCService) RevokeGrant(userID, grantID string) error {
	now := s.now().UTC()
	result := s.db.Model(&models.OAuthGrant{}).Where("id = ? AND user_id = ? AND revoked_at IS NULL", grantID, userID).Update("revoked_at", now)
	if result.Error != nil || result.RowsAffected != 1 {
		return fmt.Errorf("grant not found")
	}
	return nil
}

func registeredRedirect(client models.OIDCClient, candidate string) bool {
	var redirects []string
	if json.Unmarshal([]byte(client.RedirectURIs), &redirects) != nil {
		return false
	}
	for _, registered := range redirects {
		if registered == candidate && ValidRedirectURI(registered) {
			return true
		}
	}
	return false
}

func ValidRedirectURI(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Fragment != "" || parsed.Host == "" {
		return false
	}
	if parsed.Scheme == "https" {
		return true
	}
	if parsed.Scheme != "http" {
		return false
	}
	host := parsed.Hostname()
	ip := net.ParseIP(host)
	return host == "localhost" || (ip != nil && ip.IsLoopback())
}

func validateScopes(requested, allowed string, requireOpenID bool) (string, error) {
	allowedSet := make(map[string]bool)
	for _, scope := range strings.Fields(allowed) {
		allowedSet[scope] = true
	}
	var accepted []string
	seen := make(map[string]bool)
	for _, scope := range strings.Fields(requested) {
		if !supportedScopes[scope] || !allowedSet[scope] || seen[scope] {
			return "", fmt.Errorf("invalid or duplicate scope")
		}
		seen[scope] = true
		accepted = append(accepted, scope)
	}
	if requireOpenID && !seen["openid"] {
		return "", fmt.Errorf("openid scope is required")
	}
	return strings.Join(accepted, " "), nil
}
