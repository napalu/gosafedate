//go:build windows

package self

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/napalu/gosafedate/internal/crypto"
	"github.com/napalu/gosafedate/metadata"
)

const (
	envUpdateHelper = "GOSAFEDATE_UPDATE_HELPER"
	envAutoRestart  = "GOSAFEDATE_AUTO_RESTART"
	envOrigArgs     = "GOSAFEDATE_ORIG_ARGS" // JSON []string

	newSuffix  = ".new"
	metaSuffix = ".meta"
)

var (
	execCmd   = exec.Command
	verifyRaw = crypto.VerifyRaw
)

// MaybeRunUpdateHelper should be called early in main() on Windows.
//
// It uses only:
//   - the location of the current executable (os.Executable),
//   - the ".new.meta" metadata written by the updater,
//   - the embedded Ed25519 public key (passed in by the caller)
//
// and it performs:
//
//  1. Load metadata from "<exe>.meta"
//  2. Re-verify checksum of <exe> against metadata.sha256
//  3. Re-verify Ed25519 signature over "version+sha256"
//  4. Wait until "<exe without .new>" is replacable
//  5. Atomically rename "<exe>" -> "<exe without .new>"
//  6. Optionally restart "<exe without .new>" with original args
//  7. Remove "<exe>.meta" and exit
//
// If not in helper mode, MaybeRunUpdateHelper returns immediately.
// On non-Windows platforms, a stub exists so it is safe to call
// unconditionally from main().
func MaybeRunUpdateHelper(pubKey []byte) {
	if os.Getenv(envUpdateHelper) != "1" {
		return
	}
	if err := runUpdateHelper(pubKey); err != nil {
		// in production, just treat any error as fatal for the helper
		os.Exit(1)
	}
	os.Exit(0)
}

// replaceBinary on Windows does NOT rename directly, because the running
// executable is usually locked. Instead it:
//   - renames tmpNewPath -> oldPath+".new"
//   - writes metadata to oldPath+".new.meta"
//   - launches oldPath+".new" in "helper mode"
//
// The helper will wait for the old exe to be unlocked, verify metadata
// again, perform an atomic rename, and optionally restart the app.
//
// If the process does not have write permission to the install directory,
// this will return an error (ACCESS_DENIED on Program Files etc.).
func replaceBinary(cfg Config, oldPath, tmpNewPath string, m *metadata.Metadata) error {
	absOld, err := filepath.Abs(oldPath)
	if err != nil {
		return fmt.Errorf("resolve oldPath: %w", err)
	}
	absTmp, err := filepath.Abs(tmpNewPath)
	if err != nil {
		return fmt.Errorf("resolve newPath: %w", err)
	}

	newPath := absOld + newSuffix
	metaPath := newPath + metaSuffix

	// original process moves temp → .new
	if err := rename(absTmp, newPath); err != nil {
		return fmt.Errorf("rename %q -> %q: %w", absTmp, newPath, err)
	}

	metaBytes, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, metaBytes, 0o600); err != nil {
		return fmt.Errorf("write metadata %q: %w", metaPath, err)
	}

	env := os.Environ()
	env = append(env,
		envUpdateHelper+"=1",
	)

	autoRestart := "0"
	if cfg.AutoRestart {
		autoRestart = "1"
	}
	env = append(env, envAutoRestart+"="+autoRestart)

	if b, err := json.Marshal(os.Args[1:]); err == nil {
		env = append(env, envOrigArgs+"="+string(b))
	}

	cmd := execCmd(newPath)
	cmd.Env = env

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting update helper: %w", err)
	}

	return nil
}

// restartBinary is a no-op on Windows; restart is handled by the helper.
func restartBinary(_ string) error {
	return nil
}

// runUpdateHelper is called by MaybeRunUpdateHelper on Windows.
func runUpdateHelper(pubKey []byte) error {
	exePath, err := executable()
	if err != nil {
		return err
	}
	exePath, _ = filepath.Abs(exePath)

	if !strings.HasSuffix(exePath, newSuffix) {
		return fmt.Errorf("not a helper exe (no %s suffix)", newSuffix)
	}
	oldPath := strings.TrimSuffix(exePath, newSuffix)
	metaPath := exePath + metaSuffix

	metaBytes, err := os.ReadFile(metaPath)
	if err != nil {
		return err
	}

	var m metadata.Metadata
	if err := json.Unmarshal(metaBytes, &m); err != nil {
		return err
	}

	f, err := os.Open(exePath)
	if err != nil {
		return err
	}
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		f.Close()
		return err
	}
	f.Close()
	sum := fmt.Sprintf("%x", h.Sum(nil))
	if !strings.EqualFold(sum, m.Checksum) {
		return fmt.Errorf("checksum mismatch: %s != %s", sum, m.Checksum)
	}

	ok, err := verifyRaw(pubKey, fmt.Sprintf("%s+%s", m.Version, m.Checksum), m.Signature)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("signature verification failed")
	}

	var lastErr error
	for i := 0; i < 100; i++ {
		if err := rename(exePath, oldPath); err == nil {
			lastErr = nil
			break
		} else {
			lastErr = err
			time.Sleep(200 * time.Millisecond)
		}
	}
	if lastErr != nil {
		return lastErr
	}

	_ = os.Remove(metaPath)

	if os.Getenv(envAutoRestart) == "1" {
		var args []string
		if raw := os.Getenv(envOrigArgs); raw != "" {
			_ = json.Unmarshal([]byte(raw), &args)
		}
		cmd := execCmd(oldPath, args...)
		_ = cmd.Start()
	}

	return nil
}
