// Package otel serves auxiliary OpenTelemetry and PromQL HTTP routes.
package otel

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hashicorp/go-hclog"

	"github.com/hashicorp-forge/hermes/internal/config"
)

const defaultListenerAddr = "127.0.0.1:9464"

// ListenerAddr returns the configured OTEL listener address.
func ListenerAddr(cfg *config.OpenTelemetry) string {
	if cfg == nil || strings.TrimSpace(cfg.Addr) == "" {
		return defaultListenerAddr
	}
	return cfg.Addr
}

// Handler returns a Gin-backed HTTP handler for OTEL auxiliary routes.
func Handler(cfg *config.OpenTelemetry, log hclog.Logger) http.Handler {
	if log == nil {
		log = hclog.NewNullLogger()
	}

	r := gin.New()
	r.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.GET("/api/v2/otel/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"ok": true})
	})
	r.Any("/api/v2/otel/promql/query", promQLProxyHandler(cfg, log, "/api/v1/query"))
	r.Any("/api/v2/otel/promql/query_range", promQLProxyHandler(cfg, log, "/api/v1/query_range"))
	return r
}

func promQLProxyHandler(cfg *config.OpenTelemetry, log hclog.Logger, upstreamPath string) gin.HandlerFunc {
	client := &http.Client{Timeout: 30 * time.Second}
	return func(c *gin.Context) {
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodPost {
			c.Status(http.StatusMethodNotAllowed)
			return
		}

		if cfg == nil || strings.TrimSpace(cfg.PrometheusURL) == "" {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Prometheus URL is not configured"})
			return
		}

		upstreamURL, err := buildPromQLURL(cfg.PrometheusURL, upstreamPath, c.Request.URL.RawQuery)
		if err != nil {
			log.Error("error building PromQL upstream URL", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Invalid Prometheus URL"})
			return
		}

		req, err := http.NewRequestWithContext(c.Request.Context(), c.Request.Method, upstreamURL, c.Request.Body)
		if err != nil {
			log.Error("error creating PromQL upstream request", "error", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error creating upstream request"})
			return
		}
		copyPromQLHeaders(req.Header, c.Request.Header)

		resp, err := client.Do(req)
		if err != nil {
			log.Error("error proxying PromQL request", "error", err)
			c.JSON(http.StatusBadGateway, gin.H{"error": "Error querying Prometheus"})
			return
		}
		defer func() { _ = resp.Body.Close() }()

		for key, values := range resp.Header {
			for _, value := range values {
				c.Writer.Header().Add(key, value)
			}
		}
		c.Status(resp.StatusCode)
		if _, err := io.Copy(c.Writer, resp.Body); err != nil {
			log.Error("error writing PromQL response", "error", err)
		}
	}
}

func buildPromQLURL(baseURL, upstreamPath, rawQuery string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base URL: %w", err)
	}
	u.Path = strings.TrimRight(u.Path, "/") + upstreamPath
	u.RawQuery = rawQuery
	return u.String(), nil
}

func copyPromQLHeaders(dst, src http.Header) {
	for key, values := range src {
		if strings.EqualFold(key, "Host") {
			continue
		}
		for _, value := range values {
			dst.Add(key, value)
		}
	}
}
