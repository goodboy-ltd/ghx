package release

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
)

const testLimit = 16

// serve answers /<name> with body. A chunked body is sent without a Content-Length.
func serve(t *testing.T, files map[string][]byte, chunked bool) (Client, string) {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, ok := files[strings.TrimPrefix(r.URL.Path, "/")]
		if !ok {
			http.NotFound(w, r)
			return
		}
		if chunked {
			_, _ = w.Write(body[:1])
			w.(http.Flusher).Flush()
			_, _ = w.Write(body[1:])
			return
		}
		w.Header().Set("Content-Length", strconv.Itoa(len(body)))
		_, _ = w.Write(body)
	}))
	t.Cleanup(server.Close)
	return Client{http: server.Client(), maxBytes: testLimit}, server.URL + "/"
}

func TestGetEnforcesLimit(t *testing.T) {
	cases := []struct {
		name    string
		size    int
		chunked bool
		tooBig  bool
	}{
		{"at the limit", testLimit, false, false},
		{"at the limit, chunked", testLimit, true, false},
		{"over the limit", testLimit + 1, false, true},
		{"over the limit, chunked", testLimit + 1, true, true},
		{"far over the limit, chunked", 10 * testLimit, true, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			body := bytes.Repeat([]byte("x"), tc.size)
			client, base := serve(t, map[string][]byte{"asset": body}, tc.chunked)
			got, err := client.get(base + "asset")
			if !tc.tooBig {
				if err != nil || !bytes.Equal(got, body) {
					t.Fatalf("got %d bytes, %v", len(got), err)
				}
				return
			}
			if !errors.Is(err, ErrTooLarge) || got != nil {
				t.Fatalf("got %d bytes, %v; want ErrTooLarge", len(got), err)
			}
			message := err.Error()
			if !strings.Contains(message, "too large") || !strings.Contains(message, strconv.Itoa(testLimit)) {
				t.Fatalf("error %q should say too large and name the limit", message)
			}
		})
	}
}

func TestDownloadVerifiedReportsTooLargeNotMismatch(t *testing.T) {
	asset := bytes.Repeat([]byte("x"), 200)
	digest := sha256.Sum256(asset)
	checksums := []byte(hex.EncodeToString(digest[:]) + "  gh.tar.gz\n")
	client, base := serve(t, map[string][]byte{"gh.tar.gz": asset, "checksums.txt": checksums}, true)
	// The checksums file fits exactly; the asset does not. Before ErrTooLarge, the truncated
	// asset failed with a checksum mismatch.
	client.maxBytes = int64(len(checksums))
	_, err := client.DownloadVerified(Asset{Name: "gh.tar.gz", DownloadURL: base + "gh.tar.gz"},
		Asset{Name: "checksums.txt", DownloadURL: base + "checksums.txt"})
	if !errors.Is(err, ErrTooLarge) {
		t.Fatalf("got %v, want ErrTooLarge", err)
	}
}

func TestClientDefaultLimit(t *testing.T) {
	if got := (Client{}).limit(); got != maxAssetBytes {
		t.Fatalf("got %d, want %d", got, maxAssetBytes)
	}
}
