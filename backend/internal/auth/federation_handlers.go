package auth

import (
	"errors"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/baharilink/baharimrv/internal/httpx"
	"github.com/gin-gonic/gin"
)

// FederationHandler turns an external BahariLink authorization code into a
// sovereign BahariMRV session, or binds it to the account already signed in.
type FederationHandler struct {
	service *FederationService
	clients map[string]string
	// slugs lets a client name an issuer in the path (/federation/jahazi/exchange)
	// without ever learning an unconfigured URL.
	slugs map[string]string
}

func NewFederationHandler(service *FederationService, clients map[string]string) *FederationHandler {
	handler := &FederationHandler{service: service, clients: clients, slugs: make(map[string]string, len(clients))}
	for issuer := range clients {
		if slug, ok := issuerSlug(issuer); ok {
			handler.slugs[slug] = issuer
		}
	}
	return handler
}

// Slug resolves a path segment such as "jahazi" to its configured issuer URL.
func (h *FederationHandler) Slug(segment string) (string, bool) {
	issuer, ok := h.slugs[strings.ToLower(strings.TrimSpace(segment))]
	return issuer, ok
}

func issuerSlug(issuer string) (string, bool) {
	host, _, found := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(issuer, "https://"), "http://"), "/")
	if !found || host == "" {
		return "", false
	}
	label := strings.Split(host, ".")[0]
	if label == "" || !regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,62}$`).MatchString(label) {
		return "", false
	}
	return label, true
}

type federationRequest struct {
	Issuer       string `json:"issuer"`
	Code         string `json:"code"`
	CodeVerifier string `json:"codeVerifier"`
	Nonce        string `json:"nonce"`
	RedirectURI  string `json:"redirectUri"`
}

func (h *FederationHandler) resolveIssuer(requested string) (string, bool) {
	if requested != "" {
		requested = strings.TrimRight(requested, "/")
		_, ok := h.clients[requested]
		return requested, ok
	}
	if len(h.clients) != 1 {
		return "", false
	}
	for issuer := range h.clients {
		return issuer, true
	}
	return "", false
}

func (h *FederationHandler) redeem(c *gin.Context) (issuer, idToken, nonce string, ok bool) {
	var request federationRequest
	if !httpx.BindJSON(c, &request) {
		return "", "", "", false
	}
	named := strings.TrimSpace(c.Param("issuer"))
	if named != "" {
		var trusted bool
		issuer, trusted = h.Slug(named)
		if !trusted {
			httpx.Error(c, http.StatusForbidden, "untrusted_issuer", "This federated issuer is not configured for BahariMRV.")
			return "", "", "", false
		}
		if request.Issuer != "" {
			if resolved, trusted := h.resolveIssuer(strings.TrimSpace(request.Issuer)); !trusted || resolved != issuer {
				httpx.Error(c, http.StatusForbidden, "untrusted_issuer", "The requested issuer does not match this endpoint.")
				return "", "", "", false
			}
		}
	} else {
		var trusted bool
		issuer, trusted = h.resolveIssuer(strings.TrimSpace(request.Issuer))
		if !trusted {
			httpx.Error(c, http.StatusForbidden, "untrusted_issuer", "This federated issuer is not configured for BahariMRV.")
			return "", "", "", false
		}
	}
	if request.Code == "" || request.CodeVerifier == "" || request.Nonce == "" {
		httpx.Error(c, http.StatusBadRequest, "invalid_authorization_response", "The authorization code, PKCE verifier and nonce are all required.")
		return "", "", "", false
	}
	redirect, err := url.Parse(strings.TrimSpace(request.RedirectURI))
	if err != nil || !validRedirectScheme(redirect) {
		httpx.Error(c, http.StatusBadRequest, "invalid_redirect_uri", "The redirect URI must be HTTPS, or plain HTTP on localhost.")
		return "", "", "", false
	}
	idToken, err = h.service.validator.ExchangeCode(c.Request.Context(), issuer, request.Code, request.CodeVerifier, redirect.String())
	if err != nil {
		httpx.Error(c, http.StatusBadGateway, "federation_exchange_failed", "The federated issuer rejected the code exchange. No session was created.")
		return "", "", "", false
	}
	return issuer, idToken, request.Nonce, true
}

func validRedirectScheme(redirect *url.URL) bool {
	if redirect.Host == "" {
		return false
	}
	if redirect.Scheme == "https" {
		return true
	}
	host := strings.ToLower(redirect.Hostname())
	return redirect.Scheme == "http" && (host == "localhost" || host == "127.0.0.1" || host == "::1")
}

func (h *FederationHandler) Exchange(c *gin.Context) {
	_, idToken, nonce, ok := h.redeem(c)
	if !ok {
		return
	}
	result, err := h.service.Login(c.Request.Context(), idToken, nonce)
	switch {
	case errors.Is(err, ErrInvalidCredentials):
		httpx.Error(c, http.StatusForbidden, "federation_identity_invalid", "The federated identity could not be verified.")
	case err != nil:
		httpx.Error(c, http.StatusForbidden, "identity_not_active", "IOTA identity provisioning must complete before this account can sign in.")
	default:
		c.JSON(http.StatusOK, result)
	}
}

func (h *FederationHandler) Link(c *gin.Context) {
	_, idToken, nonce, ok := h.redeem(c)
	if !ok {
		return
	}
	link, err := h.service.Link(c.Request.Context(), c.GetString("user_id"), idToken, nonce)
	if err != nil {
		httpx.Error(c, http.StatusConflict, "link_failed", "This Jahazi identity is already linked to another BahariMRV account.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"issuer": link.Issuer, "subject": link.Subject, "status": "linked"})
}

func (h *FederationHandler) Links(c *gin.Context) {
	links, err := h.service.Links(c.GetString("user_id"))
	if err != nil {
		httpx.Error(c, http.StatusInternalServerError, "links_unavailable", "Linked platforms could not be read.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"links": links})
}

func (h *FederationHandler) Unlink(c *gin.Context) {
	if err := h.service.Unlink(c.GetString("user_id"), c.Param("linkId")); err != nil {
		httpx.Error(c, http.StatusNotFound, "link_not_found", "The link was not found for this account.")
		return
	}
	c.Status(http.StatusNoContent)
}
