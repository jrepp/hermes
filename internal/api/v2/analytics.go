package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/hashicorp-forge/hermes/internal/server"
)

// AnalyticsRequest represents a user analytics event request.
type AnalyticsRequest struct {
	DocumentID  string `json:"document_id"`
	ProductName string `json:"product_name"`
}

// AnalyticsResponse represents the response from an analytics event.
type AnalyticsResponse struct {
	Recorded bool `json:"recorded"`
}

// AnalyticsHandler returns an HTTP handler for analytics events.
//
// @Summary Record web analytics event
// @Description Records frontend analytics events such as document views.
// @Tags web
// @Accept json
// @Produce json
// @Param request body AnalyticsRequest true "Analytics event"
// @Success 200 {object} AnalyticsResponse
// @Failure 400 {string} string "Error decoding analytics request"
// @Failure 405 {string} string "method not allowed"
// @Router /api/v2/web/analytics [post]
// @Security UserAuth
// @x-rbac {"resource":"web.analytics","action":"create","authenticated":true}
func AnalyticsHandler(srv server.Server) http.Handler {
	r := gin.New()
	r.Any("/*path", ginAnalyticsHandler(srv))
	return r
}

func ginAnalyticsHandler(srv server.Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodPost {
			c.Header("Allow", http.MethodPost)
			c.JSON(http.StatusMethodNotAllowed, ErrorResponse{
				Error:   c.Request.Method + " is not allowed here; use POST",
				Path:    c.Request.URL.Path,
				Allowed: []string{http.MethodPost},
			})
			return
		}

		var req AnalyticsRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			srv.Logger.Error("error decoding analytics request", "error", err)
			c.String(http.StatusBadRequest, "Error decoding analytics request")
			return
		}

		response := &AnalyticsResponse{
			Recorded: false,
		}

		// Check if document id is set, product name is optional
		if req.DocumentID != "" {
			srv.Logger.Info(
				"document view event",
				"method", c.Request.Method,
				"path", c.Request.URL.Path,
				"document_id", req.DocumentID,
				"product_name", req.ProductName,
			)
			response.Recorded = true
		}

		c.JSON(http.StatusOK, response)
	}
}
