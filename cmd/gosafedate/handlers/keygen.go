package handlers

import (
	"fmt"
	"path/filepath"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/gosafedate/cmd/gosafedate/config"
	"github.com/napalu/gosafedate/internal/crypto"
)

// HandleKeygen creates an Ed25519 key pair.
func HandleKeygen(p *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*config.Config](p)
	if !ok {
		return fmt.Errorf("failed to get options from context")
	}

	priv := cfg.Keygen.Prefix
	pub := cfg.Keygen.Prefix + ".pub"

	if err := crypto.GenerateKeys(priv, pub); err != nil {
		return fmt.Errorf("keygen failed: %w", err)
	}

	fmt.Printf("✅ Generated key pair:\n  %s\n  %s\n", filepath.Base(priv), filepath.Base(pub))
	return nil
}
