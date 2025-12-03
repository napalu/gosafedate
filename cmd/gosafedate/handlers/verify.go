package handlers

import (
	"fmt"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/gosafedate/cmd/gosafedate/config"
	"github.com/napalu/gosafedate/internal/crypto"
)

func HandleVerify(p *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*config.Config](p)
	if !ok {
		return fmt.Errorf("failed to get options from context")
	}

	valid, err := crypto.VerifyFile(cfg.Verify.PubPath, cfg.Verify.Message, cfg.Verify.Signature)
	if err != nil {
		return fmt.Errorf("verify failed: %w", err)
	}
	if !valid {
		return fmt.Errorf("invalid signature")
	}

	fmt.Println("valid signature")
	return nil
}
