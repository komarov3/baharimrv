package auth

import (
	"net/http"

	"github.com/baharilink/baharimrv/internal/httpx"
	"github.com/gin-gonic/gin"
)

type OIDCHandler struct{ service *OIDCService }

func NewOIDCHandler(service *OIDCService) *OIDCHandler { return &OIDCHandler{service: service} }

func (h *OIDCHandler) Discovery(c *gin.Context) { c.JSON(http.StatusOK, h.service.Discovery()) }
func (h *OIDCHandler) JWKS(c *gin.Context)      { c.JSON(http.StatusOK, h.service.signer.JWKS()) }

func (h *OIDCHandler) Authorize(c *gin.Context) {
	params := AuthorizeParams{
		ClientID: c.Query("client_id"), RedirectURI: c.Query("redirect_uri"),
		ResponseType: c.Query("response_type"), Scope: c.Query("scope"),
		State: c.Query("state"), Nonce: c.Query("nonce"),
		CodeChallenge: c.Query("code_challenge"), CodeChallengeMethod: c.Query("code_challenge_method"),
	}
	target, err := h.service.Authorize(c.GetString("user_id"), params)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_authorization_request", "The authorization request is invalid or the grant is unavailable.")
		return
	}
	c.Redirect(http.StatusFound, target)
}

func (h *OIDCHandler) Token(c *gin.Context) {
	result, err := h.service.Token(
		c.PostForm("client_id"), c.PostForm("client_secret"), c.PostForm("grant_type"),
		c.PostForm("code"), c.PostForm("code_verifier"), c.PostForm("redirect_uri"),
	)
	if err != nil {
		c.Header("Cache-Control", "no-store")
		httpx.Error(c, http.StatusBadRequest, "invalid_grant", "The authorization grant, client authentication, redirect, or PKCE verifier is invalid.")
		return
	}
	c.Header("Cache-Control", "no-store")
	c.Header("Pragma", "no-cache")
	c.JSON(http.StatusOK, result)
}

func (h *OIDCHandler) Userinfo(c *gin.Context) {
	claims, err := h.service.Userinfo(bearerToken(c))
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, "invalid_token", "A valid, unrevoked access token is required.")
		return
	}
	c.JSON(http.StatusOK, claims)
}

func (h *OIDCHandler) RevokeGrant(c *gin.Context) {
	var request struct {
		GrantID string `json:"grant_id"`
	}
	if !httpx.BindJSON(c, &request) {
		return
	}
	if err := h.service.RevokeGrant(c.GetString("user_id"), request.GrantID); err != nil {
		httpx.Error(c, http.StatusNotFound, "grant_not_found", "The grant was not found.")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *OIDCHandler) RegisterClient(c *gin.Context) {
	var request struct {
		ClientID     string   `json:"client_id"`
		ClientSecret string   `json:"client_secret"`
		RedirectURIs []string `json:"redirect_uris"`
		Scopes       string   `json:"scopes"`
	}
	if !httpx.BindJSON(c, &request) {
		return
	}
	client, err := h.service.RegisterClient(request.ClientID, request.ClientSecret, request.RedirectURIs, request.Scopes)
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_client", "The client registration is invalid.")
		return
	}
	c.JSON(http.StatusCreated, client)
}
