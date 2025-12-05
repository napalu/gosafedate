// Package self provides a minimal, secure self-update mechanism for Go binaries.
// It validates updates using SHA-256 + Ed25519 signatures, performs atomic
// replacements, and exposes both a simple and modular API.

package self

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/napalu/gosafedate/internal/crypto"
	"github.com/napalu/gosafedate/metadata"
	"github.com/napalu/gosafedate/version"
)

type Config struct {
	AutoRestart bool
	URL         string
	PubKey      []byte
	CurrentVer  string
	TargetPath  string  // if empty: use os.Executable()
	LogInfo     LogFunc // optional logger hook
	LogError    LogFunc // optional logger hook
}

type LogFunc func(string, ...interface{})

var httpGet = http.Get
var execSelf = syscall.Exec
var executable = os.Executable
var rename = os.Rename

// HasNewer checks remote metadata and returns whether a newer version
// than cfg.CurrentVer is available. If true, it also returns the
// parsed metadata used for the decision.
func HasNewer(cfg Config) (bool, *metadata.Metadata, error) {
	logInfo, logError := normalizeLogs(cfg)
	logInfo("checking for updates...")

	if cfg.URL == "" {
		logInfo("no update URL found - can't check")
		return false, nil, nil
	}

	m, err := fetchMetadata(cfg.URL)
	if err != nil {
		logError("failed to fetch metadata: %v", err)
		return false, nil, err
	}

	newer, err := shouldUpdate(cfg.CurrentVer, m)
	if err != nil {
		logError("failed to determine if we should update version: %v", err)
		return false, nil, err
	}

	if !newer {
		logInfo("no new version found - skipping update")
	}

	return newer, m, nil
}

// UpdateIfNewer checks for a newer version using the provided metadata URL.
// If a verified update is available, it atomically replaces the current
// executable and, if AutoRestart is true, re-executes the process.
// If already up to date, it simply returns nil.
func UpdateIfNewer(cfg Config) error {
	newer, m, err := HasNewer(cfg)
	if err != nil || !newer {
		return err
	}

	return UpdateFromMetadata(cfg, m)
}

// UpdateFromMetadata atomically replaces the current executable with a new
// version downloaded from the provided metadata URL.
func UpdateFromMetadata(cfg Config, m *metadata.Metadata) error {
	var err error
	logInfo, logError := normalizeLogs(cfg)

	if m == nil || cfg.CurrentVer == m.Version {
		return nil
	}

	logInfo("updating from %s to %s", cfg.CurrentVer, m.Version)

	var currPath string
	if cfg.TargetPath != "" {
		currPath = cfg.TargetPath
	} else {
		currPath, err = executable()
		if err != nil {
			logError("failed to determine current executable path: %v", err)
			return err
		}
	}
	curFile := filepath.Base(currPath)
	downloadFile := filepath.Join(filepath.Dir(currPath), fmt.Sprintf("%s-%s.gz", curFile, m.Version))

	logInfo("downloading")

	resolvedURL, err := resolveURL(cfg.URL, m.DownloadURL)
	if err != nil {
		logError("failed to resolve download URL: %v", err)
		return err
	}

	if err = fetchAndDownload(resolvedURL, downloadFile); err != nil {
		logError("failed to download update: %v", err)
		return err
	}

	defer os.Remove(downloadFile)

	gzipFile, err := os.Open(downloadFile)
	if err != nil {
		logError("failed to open update file: %v", err)
		return err
	}
	defer gzipFile.Close()

	gzipReader, err := gzip.NewReader(gzipFile)
	if err != nil {
		logError("failed to create gzip reader: %v", err)
		return err
	}
	defer gzipReader.Close()

	uncompressedFile, err := os.Create(strings.TrimSuffix(downloadFile, ".gz"))
	if err != nil {
		logError("failed to create uncompressed file: %v", err)
		return err
	}
	defer uncompressedFile.Close()

	_, err = io.Copy(uncompressedFile, gzipReader)
	if err != nil {
		logError("failed to decompress update: %v", err)
		return err
	}

	logInfo("verifying checksum")
	err = verifyChecksum(uncompressedFile.Name(), m)
	if err != nil {
		logError("failed to verify checksum: %v", err)
		return err
	}

	if len(cfg.PubKey) > 0 {
		logInfo("verifying signature")
		ok, err := crypto.VerifyRaw(cfg.PubKey, fmt.Sprintf("%s+%s", m.Version, m.Checksum), m.Signature)
		if err != nil {
			logError("failed to verify signature: %v", err)
			return err
		}
		if !ok {
			err = fmt.Errorf("signature verification failed")
			logError(err.Error())
			return err
		}
	}

	if err = uncompressedFile.Sync(); err != nil {
		logError("failed to sync new binary to disk: %v", err)
		return err
	}

	oldInfo, err := os.Stat(currPath)
	if err != nil {
		logError("failed to stat current executable: %v", err)
		return err
	}
	oldMode := oldInfo.Mode()

	if err = replaceBinary(cfg, currPath, uncompressedFile.Name(), m); err != nil {
		logError("failed to update: %v", err)
		return err
	}

	if err = restorePermissions(currPath, oldMode); err != nil {
		logError("failed to make file executable: %v", err)
	}

	if cfg.AutoRestart {
		logInfo("restarting")

		// Explicit cleanup before os.Exit since defers won't run
		// Ignore errors here; process is about to exit.
		_ = gzipReader.Close()
		_ = gzipFile.Close()
		_ = os.Remove(downloadFile)
		_ = uncompressedFile.Close()

		if err = restartBinary(currPath); err != nil {
			logError("failed to restart: %v", err)
			return err
		}

		os.Exit(0)
	}

	logInfo("update installed, please restart manually")
	return nil
}

func restorePermissions(path string, mode os.FileMode) error {
	return os.Chmod(path, mode)
}

func fetchMetadata(url string) (*metadata.Metadata, error) {
	resp, err := httpGet(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("metadata HTTP %d", resp.StatusCode)
	}

	var m metadata.Metadata
	if err = json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func fetchAndDownload(url, dest string) error {
	resp, err := httpGet(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download HTTP %d", resp.StatusCode)
	}

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}

func verifyChecksum(path string, m *metadata.Metadata) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	h := sha256.New()
	if _, err = io.Copy(h, file); err != nil {
		return err
	}

	sum := fmt.Sprintf("%x", h.Sum(nil))
	if !strings.EqualFold(sum, m.Checksum) {
		return fmt.Errorf("checksum mismatch for %s != %s", sum, m.Checksum)
	}

	return nil
}

func shouldUpdate(currentVersion string, metadata *metadata.Metadata) (bool, error) {
	if currentVersion == "" || strings.Contains(currentVersion, "dev") {
		return false, nil
	}

	cv, err := version.NewSemVer(currentVersion, "v")
	if err != nil {
		return false, err
	}
	nv, err := version.NewSemVer(metadata.Version, "v")
	if err != nil {
		return false, err
	}

	return nv.GreaterThan(cv), nil
}

func resolveURL(metaURL, downloadURL string) (string, error) {
	du, err := url.Parse(downloadURL)
	if err != nil {
		return "", err
	}
	if du.IsAbs() {
		return downloadURL, nil
	}
	mu, err := url.Parse(metaURL)
	if err != nil {
		return "", err
	}
	return mu.ResolveReference(du).String(), nil
}

func normalizeLogs(c Config) (logInfo, logError LogFunc) {
	if c.LogInfo == nil {
		logInfo = func(string, ...interface{}) { /* be quiet */ }
	} else {
		logInfo = c.LogInfo
	}

	if c.LogError == nil {
		logError = func(string, ...interface{}) { /* be quiet */ }
	} else {
		logError = c.LogError
	}

	return logInfo, logError
}
