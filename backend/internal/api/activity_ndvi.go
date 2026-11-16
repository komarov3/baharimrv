package api

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func (r *router) activityNdvi(c *gin.Context) {
	if err := r.projects.AssertActivityAccess(c.GetString("user_id"), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	view, err := r.satellite.ActivityNdvi(c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

func (r *router) recaptureActivityNdvi(c *gin.Context) {
	activity, _, err := r.projects.Activity(c.Param("id"))
	if err != nil {
		fail(c, err)
		return
	}
	userID := c.GetString("user_id")
	if err := r.projects.AssertProjectManager(userID, activity.ProjectID); err != nil {
		fail(c, err)
		return
	}
	if err := r.satellite.RecaptureActivityBaseline(activity, userID); err != nil {
		fail(c, err)
		return
	}
	view, err := r.satellite.ActivityNdvi(activity.ID)
	if err != nil {
		fail(c, err)
		return
	}
	c.JSON(http.StatusAccepted, view)
}
