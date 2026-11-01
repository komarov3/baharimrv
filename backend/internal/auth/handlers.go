package auth

import (
	"errors"
	"net/http"
	"strings"

	"github.com/baharilink/baharimrv/internal/httpx"
	"github.com/baharilink/baharimrv/internal/identity"
	"github.com/baharilink/baharimrv/internal/models"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	service  *Service
	identity *identity.Service
}

func NewHandler(service *Service, identities *identity.Service) *Handler {
	return &Handler{service: service, identity: identities}
}

type registerRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DisplayName string `json:"displayName"`
	Phone       string `json:"phone"`
	Role        string `json:"role"`
}

func (h *Handler) Register(c *gin.Context) {
	var request registerRequest
	if !httpx.BindJSON(c, &request) {
		return
	}
	result, err := h.service.Register(c.Request.Context(), request.Email, request.Password, request.DisplayName, request.Phone, request.Role)
	if errors.Is(err, identity.ErrProvisioning) {
		c.JSON(http.StatusAccepted, gin.H{
			"user": result.User, "identity": result.Identity,
			"message": "Registration is recorded, but the account is not active until IOTA identity provisioning succeeds.",
		})
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "registration_failed", "Registration could not be completed with the supplied details.")
		return
	}
	c.JSON(http.StatusCreated, result)
}

func (h *Handler) Login(c *gin.Context) {
	var request struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if !httpx.BindJSON(c, &request) {
		return
	}
	result, err := h.service.Login(c.Request.Context(), request.Email, request.Password)
	if errors.Is(err, identity.ErrProvisioning) {
		httpx.Error(c, http.StatusForbidden, "identity_not_active", "IOTA identity provisioning must complete before login.")
		return
	}
	if err != nil {
		httpx.Error(c, http.StatusUnauthorized, "invalid_credentials", "The email or password is incorrect.")
		return
	}
	c.JSON(http.StatusOK, result)
}

func (h *Handler) Logout(c *gin.Context) {
	if err := h.service.Logout(bearerToken(c)); err != nil {
		httpx.Error(c, http.StatusInternalServerError, "logout_failed", "The session could not be revoked.")
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) Me(c *gin.Context) {
	user, _ := c.Get("user")
	provision, err := h.identity.ForUser(c.GetString("user_id"))
	if err != nil {
		httpx.Error(c, http.StatusNotFound, "identity_not_found", "Identity provisioning state is unavailable.")
		return
	}
	c.JSON(http.StatusOK, gin.H{"user": user, "identity": provision})
}

func (h *Handler) RetryIdentity(c *gin.Context) {
	provision, err := h.identity.EnsureProvision(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"identity": provision, "message": "Identity provisioning is still not active."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"identity": provision})
}

func (h *Handler) RecoverIdentity(c *gin.Context) {
	if !h.stepUp(c) {
		return
	}
	provision, err := h.identity.Recover(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"identity": provision, "message": "Identity recovery has not completed."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"identity": provision})
}

func (h *Handler) RotateIdentity(c *gin.Context) {
	if !h.stepUp(c) {
		return
	}
	provision, err := h.identity.Rotate(c.Request.Context(), c.GetString("user_id"))
	if err != nil {
		c.JSON(http.StatusAccepted, gin.H{"identity": provision, "message": "Identity rotation has not completed."})
		return
	}
	c.JSON(http.StatusOK, gin.H{"identity": provision})
}

func (h *Handler) stepUp(c *gin.Context) bool {
	var request struct {
		Password string `json:"password"`
	}
	if !httpx.BindJSON(c, &request) {
		return false
	}
	if !h.service.CheckPassword(c.GetString("user_id"), request.Password) {
		httpx.Error(c, http.StatusForbidden, "step_up_failed", "Recent password confirmation is required.")
		return false
	}
	return true
}

func RequireLocal(service *Service, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		token := bearerToken(c)
		if token == "" {
			httpx.Error(c, http.StatusUnauthorized, "authentication_required", "A local session is required.")
			return
		}
		user, err := service.Authenticate(token)
		if err != nil {
			httpx.Error(c, http.StatusUnauthorized, "invalid_session", "The local session is invalid or expired.")
			return
		}
		if len(roles) > 0 {
			allowed := false
			for _, role := range roles {
				if user.Role == role || user.Role == models.RoleAdmin {
					allowed = true
					break
				}
			}
			if !allowed {
				httpx.Error(c, http.StatusForbidden, "forbidden", "This role cannot perform the requested action.")
				return
			}
		}
		c.Set("user_id", user.ID)
		c.Set("user_role", user.Role)
		c.Set("user", user)
		c.Next()
	}
}

func bearerToken(c *gin.Context) string {
	parts := strings.SplitN(c.GetHeader("Authorization"), " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}
