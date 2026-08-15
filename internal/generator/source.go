package generator

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"
)

// DefaultSource is Unicode's emoji test data for the latest published release.
//
// emoji-test.txt is used in preference to emoji-sequences.txt because it names
// every emoji individually. emoji-sequences.txt collapses runs of code points
// into ranges (for example "2648..2653 ; Basic_Emoji ; Aries..Pisces"), which
// names only the endpoints and leaves the emoji in between unnamed.
const DefaultSource = "https://www.unicode.org/Public/emoji/latest/emoji-test.txt"

// open returns a reader for src, which may be an http(s) URL or a local path.
// The caller must close the returned ReadCloser.
func open(src string) (io.ReadCloser, error) {
	if !strings.HasPrefix(src, "http://") && !strings.HasPrefix(src, "https://") {
		slog.Debug("reading source from disk", "path", src)
		return os.Open(src)
	}

	slog.Debug("fetching source", "url", src)
	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(src)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", src, err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf("fetch %s: unexpected status %s", src, resp.Status)
	}
	return resp.Body, nil
}
