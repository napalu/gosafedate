package crypto_test

import (
	"path/filepath"
	"testing"

	"github.com/napalu/gosafedate/internal/crypto"
)

const testPrivKey = `-----BEGIN PRIVATE KEY-----
MC4CAQAwBQYDK2VwBCIEIL8jjLgiK19Bxqj5/eDL9raKXv2NX5QNtda4NVD6IOmS
-----END PRIVATE KEY-----`

const testPubKey = `-----BEGIN PUBLIC KEY-----
MCowBQYDK2VwAyEAh1+7KRlg1saYM8dtpZ3NkVVc5IjpO66IJxJ7m4J+2Yo=
-----END PUBLIC KEY-----`

func TestSignVerifyRoundTrip(t *testing.T) {
	data := "hello-world"

	sig, err := crypto.Sign(testPrivKey, data)
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	ok, err := crypto.Verify(testPubKey, data, sig)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if !ok {
		t.Fatalf("Verify returned false for valid signature")
	}
}

func TestVerifyDetectsTamper(t *testing.T) {
	sig, err := crypto.Sign(testPrivKey, "original")
	if err != nil {
		t.Fatalf("Sign failed: %v", err)
	}

	ok, err := crypto.Verify(testPubKey, "modified", sig)
	if err != nil {
		t.Fatalf("Verify returned error: %v", err)
	}
	if ok {
		t.Fatalf("Verify returned true for tampered data")
	}
}

func TestSignWithInvalidKey(t *testing.T) {
	_, err := crypto.Sign("not-a-key", "data")
	if err == nil {
		t.Fatalf("expected error for invalid private key, got nil")
	}
}

func TestGenerateKeysAndFileHelpers(t *testing.T) {
	dir := t.TempDir()
	priv := filepath.Join(dir, "test.key")
	pub := filepath.Join(dir, "test.key.pub")

	if err := crypto.GenerateKeys(priv, pub); err != nil {
		t.Fatalf("GenerateKeys failed: %v", err)
	}

	sig, err := crypto.SignFile(priv, "hello")
	if err != nil {
		t.Fatalf("SignFile failed: %v", err)
	}

	ok, err := crypto.VerifyFile(pub, "hello", sig)
	if err != nil {
		t.Fatalf("VerifyFile error: %v", err)
	}
	if !ok {
		t.Fatalf("VerifyFile returned false for valid signature")
	}
}
