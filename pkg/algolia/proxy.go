package algolia

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/hashicorp/go-hclog"
)

// AlgoliaProxyHandler proxies Algolia API requests from the Hermes frontend.
//
//nolint:revive // AlgoliaProxyHandler is intentional to distinguish from other proxy handlers
func AlgoliaProxyHandler(
	c *Client, cfg *Config, log hclog.Logger) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.Debug("AlgoliaProxyHandler: Received request",
			"method", r.Method,
			"path", r.URL.Path,
			"query", r.URL.RawQuery,
		)

		// Log AppID for debugging (API key is masked for security)
		log.Debug("AlgoliaProxyHandler: Using Algolia credentials",
			"AppID", c.Docs.GetAppID(),
		) // Create HTTP request.
		url := fmt.Sprintf("https://%s-dsn.algolia.net%s?%s",
			c.Docs.GetAppID(), r.URL.Path, r.URL.RawQuery)
		log.Debug("AlgoliaProxyHandler: Constructed URL", "url", url)

		client := &http.Client{
			Timeout: time.Second * 10,
		}
		//nolint:gosec // G704: url is constructed from config, not user input
		req, err := http.NewRequest(r.Method, url, r.Body)
		if err != nil {
			log.Error("AlgoliaProxyHandler: Error creating HTTP request",
				"error", err,
				"method", r.Method,
				"url", url,
				"path", r.URL.Path)
			http.Error(w, "Error executing search request",
				http.StatusInternalServerError)
			return
		}

		// Add Algolia auth headers.
		req.Header.Add("X-Algolia-API-Key", cfg.SearchAPIKey)
		req.Header.Add("X-Algolia-Application-Id", c.Docs.GetAppID())
		log.Debug("AlgoliaProxyHandler: Added Algolia auth headers")

		// Execute HTTP request.
		//nolint:gosec // G704: req is constructed from config, not user input
		resp, err := client.Do(req)
		if err != nil {
			log.Error("AlgoliaProxyHandler: Error executing search request",
				"error", err,
				"method", r.Method,
				"url", url,
				"path", r.URL.Path)
			http.Error(w, "Error executing search request",
				http.StatusInternalServerError)
			return
		}
		defer func() { _ = resp.Body.Close() }()

		// Build and write HTTP response.
		w.WriteHeader(resp.StatusCode)
		for k, v := range resp.Header {
			w.Header().Add(k, v[0])
		}
		respBody, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Error("AlgoliaProxyHandler: Error reading response body",
				"error", err,
				"status_code", resp.StatusCode,
				"method", r.Method,
				"path", r.URL.Path)
			http.Error(w, "Error reading search response",
				http.StatusInternalServerError)
			return
		}
		if _, err := w.Write(respBody); err != nil {
			log.Error("error writing response body", "error", err)
		}
	})
}
