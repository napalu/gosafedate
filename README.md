# 🛡️ gosafedate

> **Self-updates that won’t leave you on the rocks 🍸**  
> No frills, secure, signed, atomic updates for Go binaries — with zero heartbreaks.

[![Go Reference](https://pkg.go.dev/badge/github.com/napalu/gosafedate.svg)](https://pkg.go.dev/github.com/napalu/gosafedate)
[![Go Report Card](https://goreportcard.com/badge/github.com/napalu/gosafedate)](https://goreportcard.com/report/github.com/napalu/gosafedate)
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

---

### ✨ Why gosafedate?

Because `curl | bash` is not a security model.  
**gosafedate** makes it simple — and safe — for your Go apps to update themselves *without* breaking trust or atomicity.

- 🔐 **Cryptographically signed updates** using Ed25519
- 🧾 **Checksums verified** before installation
- 💣 **Atomic replacement** of the running binary
- 🚀 **Optional auto-restart**
- 💤 **Zero external dependencies**

Perfect for CLIs, daemons, or tools distributed as single binaries.

---

## 🚀 Quick Start

```bash
go get github.com/napalu/gosafedate
```

Integrating in your app:

```go
package main

import (
	"log"

	"github.com/napalu/gosafedate/self"
	"github.com/napalu/myapp/version"
)

func maybeUpdate() {
	err := self.UpdateIfNewer(self.Config{
		URL:         "https://repo.example.com/myapp/metadata.json",
		PubKey:      version.PublicKey, // raw Ed25519 key (see below)
		CurrentVer:  version.Version,
		AutoRestart: true,
	})

	if err != nil {
		log.Printf("update check failed: %v", err)
	}
}
```

## 🔑 Embedding the public key (required)

gosafedate verifies updates using the raw 32-byte Ed25519 public key, not PEM.
Generate the raw key literal from your PEM file:

```bash
gosafedate pubkey-bytes --pub myapp.key.pub
```

Example output:
```go
  []byte{0x12, 0x34, 0xab, 0xcd, /* ... */ }
```
Embed it in your app:

```go
// version/public_key.go
package version

var PublicKey = []byte{
	0x12, 0x34, 0xab, 0xcd, // ...
}
```

That’s literally all your app needs to self-update safely —
the gosafedate CLI handles everything else (keygen, signing, metadata management).

### CLI Usage

Install the CLI directly:

```bash
go install github.com/napalu/gosafedate/cmd/gosafedate@latest
```

Generate signing keys:

```bash
# Create Ed25519 key pair
gosafedate keygen myapp.key
```

Creates:
```
myapp.key
myapp.key.pub
```

#### Sign version + checksum

```bash
gosafedate sign --key myapp.key "v1.2.3+ce9f2b63e4c7e2b8..."

#### Verify signature
gosafedate verify --pub myapp.key.pub "v1.2.3+ce9f2b63e4c7e2b8..." <signature>

#### Embed public key as byte slice for compile-time inclusion
gosafedate pubkey-bytes --pub myapp.key.pub
```

### Metadata format

Each release is described by a small JSON file:

```json
{
  "version": "v1.2.3",
  "sha256": "ce9f2b63e4c7e2b8...",
  "signature": "mLr4Q1...==",
  "downloadUrl": "https://repo.example.com/myapp/myapp-v1.2.3.gz"
}
```

downloadUrl may be:
•	a full URL
•	or relative to the metadata URL (gosafedate resolves it)


### How it works
1.	Fetches metadata (metadata.json)
2.	Compares current and remote version (SemVer)
3.	Downloads and decompresses the new binary
4.	Verifies checksum and Ed25519 signature
5.	Atomically replaces the running binary
6.	Optionally restarts the process

### Trust model
* Sign once using a private key stored securely (Vault, HSM, or CI secret)
* Distribute the public key in your binary (compile-time embedding)
* gosafedate verifies checksum and signature before updating

### CI Integration Example (Jenkins)
Integrates easily with CI/CD (Jenkins, GitHub Actions, etc.)
Store your private key in Vault, and push both .gz and .json artifacts to your artifact repo (Nexus, S3, or similar).


```groovy
withCredentials([string(credentialsId: 'myapp_ed25519_key', variable: 'APP_KEY')]) {
  def hash = sha256 file: "./myapp"
  def sig = sh(script: "./myapp key sign -ad ${APP_VERSION}+${hash} -k \"$APP_KEY\"", returnStdout: true).trim()

  writeJSON(file: "./metadata.json", json: [
      version: "${APP_VERSION}",
      sha256: hash,
      downloadUrl: "https://repo.example.com/myapp/myapp-${APP_VERSION}.gz",
      signature: sig
  ])
  pushNexus("./metadata.json", "raw-go-hosted/myapp/", true)
}
```

### Dual-use

You can use gosafedate both ways:
•	as a library → build self-updating apps
•	as a CLI tool → manage signing keys and metadata in CI/CD

### Building from Source

```bash
git clone https://github.com/napalu/gosafedate.git
cd gosafedate

# Build the CLI
go build -o bin/gosafedate ./cmd/gosafedate
```

Install system-wide:

```bash

# Run tests
go test ./...
```

### Releasing

If you use make, you can define a simple workflow:

```Makefile
VERSION := $(shell git describe --tags --abbrev=0 | sed 's/^v//')
BINARY  := gosafedate

build:
	go build -trimpath -ldflags="-s -w -X github.com/napalu/gosafedate/version.Version=$(VERSION)" -o bin/$(BINARY) ./cmd/gosafedate

test:
	go test ./...

clean:
	rm -rf bin/

release: clean build
	tar czf $(BINARY)-$(VERSION).tar.gz -C bin $(BINARY)
```
Then:

```bash
make release
```
will produce `gosafedate-v1.2.3.tar.gz`

## License
MIT