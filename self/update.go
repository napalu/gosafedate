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
	TargetPath  string                       // if empty: use os.Executable()
	LogInfo     func(string, ...interface{}) // optional logger hook
	LogError    func(string, ...interface{}) // optional logger hook
}

var httpGet = http.Get
var execSelf = syscall.Exec
var executable = os.Executable
var rename = os.Rename

// UpdateIfNewer checks for a newer version using the provided metadata URL.
// If a verified update is available, it atomically replaces the current
// executable and, if AutoRestart is true, re-executes the process.
// If already up to date, it simply returns nil.
func UpdateIfNewer(cfg Config) error {
	logInfo := cfg.LogInfo
	logError := cfg.LogError
	if logInfo == nil {
		logInfo = func(s string, v ...interface{}) { /* be quiet */ }
	}
	if logError == nil {
		logError = func(s string, i ...interface{}) { /* be quiet */ }
	}
	logInfo("checking for updates...")
	if cfg.URL == "" {
		logInfo("no update URL found - can't check")
		return nil
	}

	m, err := fetchMetadata(cfg.URL)
	if err != nil {
		logError("failed to fetch metadata: ", err)
		return err
	}

	newVer, err := shouldUpdate(cfg.CurrentVer, m)
	if err != nil {
		logError("failed to determine if we should update version", err)
		return err
	}

	if !newVer {
		logInfo("no new version found - skipping update")
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
	err = fetchAndDownload(resolvedURL, downloadFile)
	if err != nil {
		logError("failed to download update", err)
		return err
	}

	defer os.Remove(downloadFile)

	gzipFile, err := os.Open(downloadFile)
	if err != nil {
		logError("failed to open update file", err)
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
		logError("failed to create uncompressed file", err)
		return err
	}
	defer uncompressedFile.Close()

	_, err = io.Copy(uncompressedFile, gzipReader)
	if err != nil {
		logError("failed to decompress update", err)
		return err
	}

	logInfo("verifying checksum")
	err = verifyChecksum(uncompressedFile.Name(), m)
	if err != nil {
		logError("failed to verify checksum", err)
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
	
	err = rename(uncompressedFile.Name(), currPath)
	if err != nil {
		logError("failed to update", err)
		return err
	}

	gzipReader.Close()
	gzipFile.Close()
	os.Remove(downloadFile)
	uncompressedFile.Close()

	if err = setExecutableBit(currPath); err != nil {
		logError("failed to make file executable", err)
	}

	if cfg.AutoRestart {
		logInfo("restarting")
		if err := restart(currPath); err != nil {
			logError("failed to restart: %v", err)
			return err
		}
		os.Exit(0)
	}

	logInfo("update installed, please restart manually")
	return nil
}

func setExecutableBit(currPath string) error {
	fileInfo, err := os.Stat(currPath)
	if err != nil {
		return err
	}
	currentPerm := fileInfo.Mode()
	newPerm := currentPerm | 0100

	return os.Chmod(currPath, newPerm)
}

func restart(currPath string) error {
	return execSelf(currPath, os.Args, os.Environ())
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
