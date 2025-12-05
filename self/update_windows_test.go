//go:build windows

package self

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/napalu/gosafedate/metadata"
)

// helper to compute sha256 hex of data
func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return fmt.Sprintf("%x", sum[:])
}

func TestRunUpdateHelper_HappyPath_NoRestart(t *testing.T) {
	// save & restore globals
	oldRename := rename
	oldExecCmd := execCmd
	oldExeFn := executable
	oldVerifyRaw := verifyRaw
	defer func() {
		rename = oldRename
		execCmd = oldExecCmd
		executable = oldExeFn
		verifyRaw = oldVerifyRaw
	}()

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "myapp.exe")
	newPath := oldPath + ".new"
	metaPath := newPath + ".meta"

	// old exe (just sentinel content)
	if err := os.WriteFile(oldPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write old exe: %v", err)
	}

	// new exe
	newData := []byte("new-binary")
	if err := os.WriteFile(newPath, newData, 0o755); err != nil {
		t.Fatalf("write new exe: %v", err)
	}

	// metadata with correct checksum
	checksum := sha256Hex(newData)
	m := metadata.Metadata{
		Version:   "v1.2.3",
		Checksum:  checksum,
		Signature: "dummy-sig",
	}
	mb, err := json.Marshal(&m)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(metaPath, mb, 0o600); err != nil {
		t.Fatalf("write meta: %v", err)
	}

	// executable() should return the helper path (the .new binary)
	executable = func() (string, error) {
		return newPath, nil
	}

	var gotFrom, gotTo string
	rename = func(from, to string) error {
		gotFrom, gotTo = from, to
		// actually perform rename to update the filesystem state
		return os.Rename(from, to)
	}

	// no restart in this test, so execCmd should not be called
	execCmd = func(name string, args ...string) *exec.Cmd {
		t.Fatalf("execCmd should not be called when autoRestart is disabled")
		return exec.Command("this-should-not-run")
	}

	// verifyRaw should be called with version+checksum and signature
	verifyRaw = func(pubKey []byte, msg, sig string) (bool, error) {
		if msg != fmt.Sprintf("%s+%s", m.Version, m.Checksum) {
			t.Fatalf("unexpected verify msg: %q", msg)
		}
		if sig != m.Signature {
			t.Fatalf("unexpected verify sig: %q", sig)
		}
		return true, nil
	}

	// ensure no autostart
	_ = os.Unsetenv(envAutoRestart)

	if err := runUpdateHelper([]byte("unused")); err != nil {
		t.Fatalf("runUpdateHelper returned error: %v", err)
	}

	// ensure rename was correct
	if gotFrom != newPath || gotTo != oldPath {
		t.Fatalf("unexpected rename: %q -> %q (expected %q -> %q)", gotFrom, gotTo, newPath, oldPath)
	}

	// oldPath must now contain new data
	got, err := os.ReadFile(oldPath)
	if err != nil {
		t.Fatalf("read updated exe: %v", err)
	}
	if !bytes.Equal(got, newData) {
		t.Fatalf("old exe not replaced with new data; got=%q", string(got))
	}

	// .meta should be removed (best-effort cleanup)
	if _, err := os.Stat(metaPath); err == nil {
		t.Fatalf("expected meta file to be removed, but it still exists")
	}
}

func TestRunUpdateHelper_HappyPath_WithRestart(t *testing.T) {
	oldRename := rename
	oldExecCmd := execCmd
	oldExeFn := executable
	oldVerifyRaw := verifyRaw
	defer func() {
		rename = oldRename
		execCmd = oldExecCmd
		executable = oldExeFn
		verifyRaw = oldVerifyRaw
	}()

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "myapp.exe")
	newPath := oldPath + ".new"
	metaPath := newPath + ".meta"

	if err := os.WriteFile(oldPath, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("write old exe: %v", err)
	}
	newData := []byte("new-binary")
	if err := os.WriteFile(newPath, newData, 0o755); err != nil {
		t.Fatalf("write new exe: %v", err)
	}

	checksum := sha256Hex(newData)
	m := metadata.Metadata{
		Version:   "v1.2.3",
		Checksum:  checksum,
		Signature: "dummy-sig",
	}
	mb, err := json.Marshal(&m)
	if err != nil {
		t.Fatalf("marshal metadata: %v", err)
	}
	if err := os.WriteFile(metaPath, mb, 0o600); err != nil {
		t.Fatalf("write meta: %v", err)
	}

	executable = func() (string, error) {
		return newPath, nil
	}

	var gotFrom, gotTo string
	rename = func(from, to string) error {
		gotFrom, gotTo = from, to
		return os.Rename(from, to)
	}

	var calledName string
	var calledArgs []string
	execCmd = func(name string, args ...string) *exec.Cmd {
		calledName = name
		calledArgs = append([]string(nil), args...)
		// return a command - we do not care what it does, just that it runs
		return exec.Command("cmd", "/c", "echo gosafedate-helper-test")
	}

	verifyRaw = func(pubKey []byte, msg, sig string) (bool, error) {
		return true, nil
	}

	// set restart env + original args
	os.Setenv(envAutoRestart, "1")
	defer os.Unsetenv(envAutoRestart)

	origArgs := []string{"-foo", "bar"}
	raw, _ := json.Marshal(origArgs)
	os.Setenv(envOrigArgs, string(raw))
	defer os.Unsetenv(envOrigArgs)

	if err := runUpdateHelper([]byte("unused")); err != nil {
		t.Fatalf("runUpdateHelper returned error: %v", err)
	}

	// check rename happened
	if gotFrom != newPath || gotTo != oldPath {
		t.Fatalf("unexpected rename: %q -> %q", gotFrom, gotTo)
	}

	// restart should have been attempted with oldPath and same args
	if calledName != oldPath {
		t.Fatalf("expected restart of %q, got %q", oldPath, calledName)
	}
	if len(calledArgs) != len(origArgs) {
		t.Fatalf("unexpected restart args: got %v, want %v", calledArgs, origArgs)
	}
	for i := range origArgs {
		if calledArgs[i] != origArgs[i] {
			t.Fatalf("unexpected restart arg[%d]: got %q, want %q", i, calledArgs[i], origArgs[i])
		}
	}
}

func TestRunUpdateHelper_ChecksumMismatch(t *testing.T) {
	oldRename := rename
	oldExecCmd := execCmd
	oldExeFn := executable
	oldVerifyRaw := verifyRaw
	defer func() {
		rename = oldRename
		execCmd = oldExecCmd
		executable = oldExeFn
		verifyRaw = oldVerifyRaw
	}()

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "myapp.exe")
	newPath := oldPath + ".new"
	metaPath := newPath + ".meta"

	_ = os.WriteFile(oldPath, []byte("old"), 0o755)
	newData := []byte("new-binary")
	if err := os.WriteFile(newPath, newData, 0o755); err != nil {
		t.Fatalf("write new exe: %v", err)
	}

	// deliberately wrong checksum
	m := metadata.Metadata{
		Version:   "v1.2.3",
		Checksum:  "0000deadbeef",
		Signature: "dummy-sig",
	}
	mb, _ := json.Marshal(&m)
	if err := os.WriteFile(metaPath, mb, 0o600); err != nil {
		t.Fatalf("write meta: %v", err)
	}

	executable = func() (string, error) { return newPath, nil }

	renameCalled := false
	rename = func(from, to string) error {
		renameCalled = true
		return errors.New("should not be called")
	}

	// verifyRaw should not matter; checksum fails earlier
	verifyRaw = func(pubKey []byte, msg, sig string) (bool, error) {
		t.Fatalf("verifyRaw should not be called on checksum mismatch")
		return false, nil
	}

	err := runUpdateHelper([]byte("unused"))
	if err == nil {
		t.Fatalf("expected error on checksum mismatch, got nil")
	}
	if renameCalled {
		t.Fatalf("rename should not be called on checksum mismatch")
	}
}

func TestRunUpdateHelper_SignatureFailure(t *testing.T) {
	oldRename := rename
	oldExecCmd := execCmd
	oldExeFn := executable
	oldVerifyRaw := verifyRaw
	defer func() {
		rename = oldRename
		execCmd = oldExecCmd
		executable = oldExeFn
		verifyRaw = oldVerifyRaw
	}()

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "myapp.exe")
	newPath := oldPath + ".new"
	metaPath := newPath + ".meta"

	_ = os.WriteFile(oldPath, []byte("old"), 0o755)
	newData := []byte("new-binary")
	if err := os.WriteFile(newPath, newData, 0o755); err != nil {
		t.Fatalf("write new exe: %v", err)
	}

	checksum := sha256Hex(newData)
	m := metadata.Metadata{
		Version:   "v1.2.3",
		Checksum:  checksum,
		Signature: "bad-sig",
	}
	mb, _ := json.Marshal(&m)
	if err := os.WriteFile(metaPath, mb, 0o600); err != nil {
		t.Fatalf("write meta: %v", err)
	}

	executable = func() (string, error) { return newPath, nil }

	renameCalled := false
	rename = func(from, to string) error {
		renameCalled = true
		return errors.New("should not be called")
	}

	// checksum OK, but signature fails
	verifyRaw = func(pubKey []byte, msg, sig string) (bool, error) {
		return false, nil
	}

	err := runUpdateHelper([]byte("unused"))
	if err == nil {
		t.Fatalf("expected error on signature failure, got nil")
	}
	if renameCalled {
		t.Fatalf("rename should not be called on signature failure")
	}
}

func TestReplaceBinary_WritesNewAndMetaAndStartsHelper(t *testing.T) {
	oldRename := rename
	oldExecCmd := execCmd
	defer func() {
		rename = oldRename
		execCmd = oldExecCmd
	}()

	dir := t.TempDir()
	oldPath := filepath.Join(dir, "myapp.exe")
	tmpNew := filepath.Join(dir, "tmp-new.exe")

	if err := os.WriteFile(oldPath, []byte("old"), 0o755); err != nil {
		t.Fatalf("write old exe: %v", err)
	}
	newData := []byte("new-binary")
	if err := os.WriteFile(tmpNew, newData, 0o755); err != nil {
		t.Fatalf("write tmp new: %v", err)
	}

	m := &metadata.Metadata{
		Version:   "v1.2.3",
		Checksum:  sha256Hex(newData),
		Signature: "dummy-sig",
	}

	var gotFrom, gotTo string
	rename = func(from, to string) error {
		gotFrom, gotTo = from, to
		return os.Rename(from, to)
	}

	var helperName string
	execCmd = func(name string, args ...string) *exec.Cmd {
		helperName = name
		return exec.Command("cmd", "/c", "echo gosafedate-helper-test")
	}

	cfg := Config{
		AutoRestart: true,
	}

	if err := replaceBinary(cfg, oldPath, tmpNew, m); err != nil {
		t.Fatalf("replaceBinary returned error: %v", err)
	}

	expectedNew := oldPath + ".new"
	expectedMeta := expectedNew + ".meta"

	if gotFrom != tmpNew || gotTo != expectedNew {
		t.Fatalf("unexpected rename in replaceBinary: %q -> %q (expected %q -> %q)", gotFrom, gotTo, tmpNew, expectedNew)
	}

	// tmpNew should be gone after rename
	if _, err := os.Stat(tmpNew); err == nil {
		t.Fatalf("expected %q to be gone after rename", tmpNew)
	}

	// meta contents should match what we passed in
	mb, err := os.ReadFile(expectedMeta)
	if err != nil {
		t.Fatalf("read meta: %v", err)
	}
	var gotMeta metadata.Metadata
	if err := json.Unmarshal(mb, &gotMeta); err != nil {
		t.Fatalf("unmarshal meta: %v", err)
	}
	if gotMeta.Version != m.Version || gotMeta.Checksum != m.Checksum || gotMeta.Signature != m.Signature {
		t.Fatalf("meta mismatch: got %+v, want %+v", gotMeta, m)
	}

	if helperName != expectedNew {
		t.Fatalf("expected helper to be started as %q, got %q", expectedNew, helperName)
	}
}
