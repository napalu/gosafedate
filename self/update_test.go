// self/update_test.go
package self

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/napalu/gosafedate/metadata"
)

// helper: gzip []byte
func gzipBytes(t *testing.T, b []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	if _, err := gz.Write(b); err != nil {
		t.Fatalf("write gzip: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func TestUpdateIfNewer_NoUpdateWhenSameVersion(t *testing.T) {
	t.Helper()

	// metadata says v1.2.3, same as current
	m := metadata.Metadata{
		Version:  "v1.2.3",
		Checksum: "deadbeef",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(m)
	}))
	defer srv.Close()

	// save & restore globals
	oldRename := rename
	oldExec := execSelf
	oldExe := executable
	defer func() {
		rename = oldRename
		execSelf = oldExec
		executable = oldExe
	}()

	// if these get called, something is wrong
	rename = func(_, _ string) error {
		t.Fatalf("rename should not be called when version is unchanged")
		return nil
	}
	execSelf = func(_ string, _ []string, _ []string) error {
		t.Fatalf("execSelf should not be called when version is unchanged")
		return nil
	}
	executable = func() (string, error) {
		return "/tmp/fake-binary", nil
	}

	err := UpdateIfNewer(Config{
		URL:        srv.URL,
		CurrentVer: "v1.2.3",
		// AutoRestart left false
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
}

func TestUpdateIfNewer_ChecksumMismatch(t *testing.T) {
	t.Helper()

	// fake new binary & gzip
	newData := []byte("new-binary")
	gz := gzipBytes(t, newData)

	// deliberate wrong checksum
	m := metadata.Metadata{
		Version:     "v1.2.4",
		Checksum:    "0000",
		DownloadURL: "/bin",
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/meta":
			_ = json.NewEncoder(w).Encode(m)
		case "/bin":
			_, _ = w.Write(gz)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()

	tmpDir := t.TempDir()
	currPath := filepath.Join(tmpDir, "myapp")
	if err := os.WriteFile(currPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write temp exe: %v", err)
	}

	oldRename := rename
	oldExec := execSelf
	oldExe := executable
	defer func() {
		rename = oldRename
		execSelf = oldExec
		executable = oldExe
	}()

	executable = func() (string, error) { return currPath, nil }
	rename = func(_, _ string) error {
		t.Fatalf("rename should not be called on checksum mismatch")
		return nil
	}
	execSelf = func(_ string, _ []string, _ []string) error {
		t.Fatalf("execSelf should not be called on checksum mismatch")
		return nil
	}

	err := UpdateIfNewer(Config{
		URL:        srv.URL + "/meta",
		CurrentVer: "v1.2.3",
		// AutoRestart false
	})
	if err == nil {
		t.Fatalf("expected error on checksum mismatch, got nil")
	}
}
