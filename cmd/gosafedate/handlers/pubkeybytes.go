package handlers

import (
	"fmt"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/gosafedate/cmd/gosafedate/config"
	"github.com/napalu/gosafedate/signing"
)

// HandlePubKeyBytes prints the []byte{} literal for embedding.
func HandlePubKeyBytes(p *goopt.Parser, _ *goopt.Command) error {
	cfg, ok := goopt.GetStructCtxAs[*config.Config](p)
	if !ok {
		return fmt.Errorf("failed to get options from context")
	}

	data, err := signing.PublicKeyFromFile(cfg.PubBytes.PubPath)
	if err != nil {
		return fmt.Errorf("failed to read pubkey: %w", err)
	}

	fmt.Print("[]byte{")
	for i, b := range data {
		if i > 0 {
			fmt.Print(", ")
		}
		fmt.Printf("0x%02x", b)
	}
	fmt.Println("}")
	return nil
}
