package generator

import (
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

// SourceURL is Unicode's emoji test data for the latest published release.
//
// emoji-test.txt is used in preference to emoji-sequences.txt because it names
// every emoji individually. emoji-sequences.txt collapses runs of code points
// into ranges (for example "2648..2653 ; Basic_Emoji ; Aries..Pisces"), which
// names only the endpoints and leaves the emoji in between unnamed.
const SourceURL = "https://www.unicode.org/Public/emoji/latest/emoji-test.txt"

// fetch retrieves the emoji data. The caller must close the reader.
func fetch() (io.ReadCloser, error) {
	slog.Debug("fetching source", "url", SourceURL)

	client := &http.Client{Timeout: 60 * time.Second}
	resp, err := client.Get(SourceURL)
	if err != nil {
		return nil, fmt.Errorf("fetch %s: %w", SourceURL, err)
	}
	if resp.StatusCode != http.StatusOK {
		_ = resp.Body.Close()
		return nil, fmt.Errorf("fetch %s: unexpected status %s", SourceURL, resp.Status)
	}
	return resp.Body, nil
}
