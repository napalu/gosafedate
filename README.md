# 🛡️ gosafedate

**Secure, signed, atomic self-updates for Go binaries — without complexity.**

[![Go Reference](https://pkg.go.dev/badge/github.com/napalu/gosafedate.svg)](https://pkg.go.dev/github.com/napalu/gosafedate)  
[![Go Report Card](https://goreportcard.com/badge/github.com/napalu/gosafedate)](https://goreportcard.com/report/github.com/napalu/gosafedate)  
[![License: MIT](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)

gosafedate provides **minimal, secure, dependency‑free** self-updates for single‑binary CLI tools and daemons.

- 🔐 **Ed25519‑signed updates**
- 🧾 **Checksum verification (SHA‑256)**
- 💣 **Atomic binary replacement**
- 🚀 **Optional auto‑restart**
- 🧘 **Zero dependencies**
- 🧱 **Tiny API surface**

Perfect when you want *safe updates without introducing a whole framework*.

---

## 🚀 Quick Start

```bash
go get github.com/napalu/gosafedate
```

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

---

## Advanced: Modular Update Flow

The API also exposes low‑level primitives so you can control the workflow:

```go
cfg := self.Config{
	URL:        "https://repo.example.com/myapp/metadata.json",
	PubKey:     version.PublicKey,
	CurrentVer: version.Version,
}

newer, meta, err := self.HasNewer(cfg)
if err != nil {
	log.Fatalf("update check failed: %v", err)
}

if newer {
	log.Printf("new version %s available", meta.Version)
	_ = self.UpdateFromMetadata(cfg, meta)
}
```

`HasNewer` only performs a remote version check and does not download anything.
`UpdateFromMetadata` performs the actual verified download and installation.

This is useful for applications that:

- want to prompt users before upgrading
- want custom logging or upgrade policies
- want to integrate UI/UX around available updates

---

## Windows: Helper Setup (required for self-update)

On Windows, a running `.exe` cannot overwrite itself.  
gosafedate therefore uses a small **helper mode** to finalize updates.

To enable this, call `MaybeRunUpdateHelper` at the very start of your `main` function:

```go
func main() {
    // On Windows: finalize any pending update
    // On all other OS: no-op
    self.MaybeRunUpdateHelper(version.PublicKey)

    // ... rest of your application ...
}
```

If you omit this call, Windows updates will download and verify correctly,
but will never be installed automatically.

On non-Windows platforms this call is a no-op and completely safe.

---
	
## Embedding the Public Key (required)

gosafedate uses **raw 32‑byte Ed25519 public keys**, not PEM.  
The CLI converts PEM → byte slice:

```bash
gosafedate pubkey-bytes --pub myapp.key.pub
```

Output:

```go
[]byte{0x12, 0x34, 0xab, 0xcd, /* ... */ }
```

Embed that into your binary:

```go
package version

var PublicKey = []byte{
	0x12, 0x34, 0xab, 0xcd, // ...
}
```

This ensures updates cannot be forged without compromising your signing key.

---

## CLI Overview

Install:

```bash
go install github.com/napalu/gosafedate/cmd/gosafedate@latest
```

### Generate signing keys

```bash
gosafedate keygen myapp.key
```

Produces:

```
myapp.key
myapp.key.pub
```

### Sign `{version}+{sha256}`

```bash
gosafedate sign --key myapp.key "v1.2.3+ce9f2b63e4c7e2b8..."
```

### Verify a signature

```bash
gosafedate verify --pub myapp.key.pub "v1.2.3+ce9f2b63e4c7e2b8..." <signature>
```

### Export raw public key bytes

```bash
gosafedate pubkey-bytes --pub myapp.key.pub
```

---

## Metadata Format

Each release is described by a small JSON file:

```json
{
  "version": "v1.2.3",
  "sha256": "ce9f2b63e4c7e2b8...",
  "signature": "mLr4Q1...==",
  "downloadUrl": "myapp-v1.2.3.gz"
}
```

`downloadUrl` may be:

- an absolute URL, or
- relative to the metadata URL:

Example:

```
https://repo.example.com/myapp/metadata.json
downloadUrl: "myapp-v1.2.3.gz"
```

Resolves to:

```
https://repo.example.com/myapp/myapp-v1.2.3.gz
```

---

## How Signing Works

gosafedate signs:

```
"{version}+{sha256}"
```

This means an attacker must compromise:

1. **The binary**, and
2. **The metadata**, and
3. **The signature**, and
4. **Your private key**

Without all four, the update is rejected.

---

## Update Flow

1. Fetch metadata
2. Parse semantic versions
3. Resolve download URL
4. Download `.gz`
5. Decompress to a temporary file
6. Verify SHA‑256
7. Verify Ed25519 signature
8. Atomically replace the running binary
9. Restore original permissions
10. Optionally restart the process

If *anything* fails: the running binary stays untouched.

---

## CI Example (Jenkins)

```groovy
withCredentials([string(credentialsId: 'myapp_ed25519_key', variable: 'APP_KEY')]) {
  def hash = sha256 file: "./myapp"
  def sig = sh(
    script: "gosafedate sign --key myapp.key \"${APP_VERSION}+${hash}\"",
    returnStdout: true
  ).trim()

  writeJSON(file: "./metadata.json", json: [
    version: "${APP_VERSION}",
    sha256: hash,
    downloadUrl: "https://repo.example.com/myapp/myapp-${APP_VERSION}.gz",
    signature: sig
  ])
}
```

---

## Compatibility & Guarantees

- **Atomic updates**: new binary fully written and verified before replacing the old one
- **Crash‑safe**: failed updates leave the current binary untouched
- **OS support**:
    - Linux, macOS, BSD: native atomic `rename(2)` replacement
    - Windows: supported via a secure helper process (no in-place overwrite)
- **Requirements**: the executable must have write permissions to its own directory

### Windows permissions

gosafedate requires that the **running process has write access to its own executable directory**.

- ✅ Per-user CLIs installed under user-writable locations
  (e.g. `%LOCALAPPDATA%\bin`) work out of the box.
- ✅ Services running as SYSTEM or a service account with write access to
  `C:\Program Files\YourApp` can self-update safely.
- ❌ GUI applications installed in `C:\Program Files` and run by standard users
  **cannot self-update** without an external elevated updater.

gosafedate does **not** attempt UAC elevation, privilege escalation,
or background services.
On permission errors, the update fails safely and the existing binary
remains untouched.

### Windows helper security model

On Windows, gosafedate never trusts environment variables for file paths.
The helper process:

- locates itself via `os.Executable()`,
- loads trusted metadata from `<exe>.new.meta`,
- re-verifies SHA-256 and Ed25519 signatures using the embedded public key,
- only then performs the final atomic rename.

This prevents hijacking or privilege escalation through crafted environment
variables or path injection.

---

## Building from Source

```bash
git clone https://github.com/napalu/gosafedate.git
cd gosafedate
go build -o bin/gosafedate ./cmd/gosafedate
```

Run tests:

```bash
go test ./...
```

---

## License

MIT  
Copyright ©

---

If you use gosafedate in your project, feel free to open an issue or PR — feedback welcome!
