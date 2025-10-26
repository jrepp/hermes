package serve

import (
	"context"
	"fmt"
	"net/http"
	"os/exec"
	"runtime"
	"time"
)

// openBrowser opens the specified URL in the user's default browser.
func openBrowser(url string) error {
	var cmd string
	var args []string

	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
		args = []string{url}
	case "linux":
		cmd = "xdg-open"
		args = []string{url}
	case "windows":
		cmd = "cmd"
		args = []string{"/c", "start", url}
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}

	return exec.Command(cmd, args...).Start()
}

// waitForServer polls the health endpoint until the server is ready or timeout.
func waitForServer(url string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	healthURL := url + "/health"
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return fmt.Errorf("timeout waiting for server to start")

		case <-ticker.C:
			resp, err := http.Get(healthURL)
			if err == nil && resp.StatusCode == http.StatusOK {
				resp.Body.Close()
				return nil
			}
			if resp != nil {
				resp.Body.Close()
			}
		}
	}
}

// printBanner displays a colorful startup banner with server information.
func printBanner(workspacePath, dbPath, indexPath, url string) {
	// ANSI color codes
	const (
		reset  = "\033[0m"
		bold   = "\033[1m"
		cyan   = "\033[36m"
		green  = "\033[32m"
		yellow = "\033[33m"
	)

	banner := fmt.Sprintf(`
%s%s╔═══════════════════════════════════════════════════════════════╗
║                                                               ║
║  %sHermes CMS - Simplified Mode%s                               ║
║                                                               ║
╚═══════════════════════════════════════════════════════════════╝%s

%s🌐 Web UI:%s       %s%s%s
%s📁 Workspace:%s    %s
%s💾 Database:%s     SQLite (embedded)
                   %s
%s🔍 Search:%s       Bleve (embedded)
                   %s

%s💡 Quick Start:%s
   • Create your first document by clicking "New Document"
   • Search is automatically indexed as you create documents
   • All data is stored locally in the docs-cms directory

%sPress Ctrl+C to stop the server%s

`,
		bold, cyan,
		green, cyan,
		reset,
		yellow, reset, bold, url, reset,
		yellow, reset, workspacePath,
		yellow, reset,
		dbPath,
		yellow, reset,
		indexPath,
		yellow, reset,
		yellow, reset,
	)

	fmt.Println(banner)
}
