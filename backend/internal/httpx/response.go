package httpx

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func Error(c *gin.Context, status int, code, message string) {
	if c.Writer.Status() != http.StatusOK && c.Writer.Written() {
		return
	}
	c.AbortWithStatusJSON(status, gin.H{
		"error":     gin.H{"code": code, "message": message},
		"requestId": c.GetString("request_id"),
	})
}

func BindJSON(c *gin.Context, target any) bool {
	if err := c.ShouldBindJSON(target); err != nil {
		Error(c, http.StatusBadRequest, "invalid_request", "The request body is invalid.")
		return false
	}
	return true
}
