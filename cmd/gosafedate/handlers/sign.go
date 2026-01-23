package handlers

import (
	"fmt"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/gosafedate/cmd/gosafedate/config"
	"github.com/napalu/gosafedate/signing"
)

func HandleSign(p *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*config.Config](p)
	if !ok {
		return fmt.Errorf("failed to get options from context")
	}

	sig, err := signing.SignFile(cfg.Sign.KeyPath, cfg.Sign.Message)
	if err != nil {
		return fmt.Errorf("sign failed: %w", err)
	}

	fmt.Println(sig)
	return nil
}
