package config

import "github.com/napalu/goopt/v2"

type Config struct {
	Keygen struct {
		Prefix string `goopt:"pos:0;required:true;desc:Prefix for key files"`
		Exec   goopt.CommandFunc
	} `goopt:"kind:command;name:keygen;desc:Generate Ed25519 keypair"`

	Sign struct {
		KeyPath string `goopt:"name:key;short:k;required:true;desc:Private key path (PEM)"`
		Message string `goopt:"pos:0;required:true;desc:Message to sign"`
		Exec    goopt.CommandFunc
	} `goopt:"kind:command;name:sign;desc:Sign a message"`

	Verify struct {
		PubPath   string `goopt:"name:pub;short:p;required:true;desc:Public key path (PEM)"`
		Message   string `goopt:"pos:0;required:true;desc:Message"`
		Signature string `goopt:"pos:1;required:true;desc:Signature (base64) to verify"`
		Exec      goopt.CommandFunc
	} `goopt:"kind:command;name:verify;desc:Verify a signature"`

	PubBytes struct {
		PubPath string `goopt:"name:pub;short:p;required:true;desc:Public key path (PEM)"`
		Exec    goopt.CommandFunc
	} `goopt:"kind:command;name:pubkey-bytes;desc:Print Go []byte literal for public key"`
}
