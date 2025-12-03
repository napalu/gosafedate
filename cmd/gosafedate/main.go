package main

import (
	"fmt"
	"log"
	"os"

	"github.com/napalu/goopt/v2"
	"github.com/napalu/gosafedate/cmd/gosafedate/config"
	"github.com/napalu/gosafedate/cmd/gosafedate/handlers"
)

func main() {
	cfg := &config.Config{}
	parser, err := goopt.NewParserFromStruct(cfg, goopt.WithExecOnParseComplete(true))
	if err != nil {
		log.Fatal(err)
	}

	// assign callbacks
	cfg.Keygen.Exec = handlers.HandleKeygen
	cfg.Sign.Exec = handlers.HandleSign
	cfg.Verify.Exec = handlers.HandleVerify
	cfg.PubBytes.Exec = handlers.HandlePubKeyBytes

	if !parser.Parse(os.Args) {
		for _, e := range parser.GetErrors() {
			_, _ = fmt.Fprintf(os.Stderr, e.Error()+"\n")
		}
		os.Exit(1)
	}
}
