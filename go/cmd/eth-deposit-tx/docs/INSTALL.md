# eth-deposit-tx — Install Guide

## Requirements

- **Go 1.21+** with CGO enabled (required for the Ledger USB HID library)
- **C toolchain** (gcc or clang) in PATH — needed to compile the Ledger USB bindings

On macOS, CGO is enabled by default and the C toolchain ships with Xcode Command Line Tools. On Linux, install `gcc` and the USB/udev development headers.

## Install from source

### From the module root

```bash
cd go/
CGO_ENABLED=1 go install ./cmd/eth-deposit-tx
```

On macOS, `CGO_ENABLED=1` is the default; you may omit it.

### Via go install (canonical)

```bash
CGO_ENABLED=1 go install github.com/rootwarp/eth-utils/go/cmd/eth-deposit-tx@latest
```

This fetches the latest tagged release and builds the binary into `$GOPATH/bin` (or `~/go/bin` by default).

### Build locally without installing

```bash
cd go/
CGO_ENABLED=1 go build -o eth-deposit-tx ./cmd/eth-deposit-tx
```

This produces an `eth-deposit-tx` binary in the current directory.

## Platform-specific notes

### macOS

CGO and a C compiler are available after installing Xcode Command Line Tools:

```bash
xcode-select --install
```

No additional USB/HID libraries are needed; macOS provides them via IOKit.

### Linux

Install `gcc`, `libusb-dev`, and `libudev-dev`:

```bash
# Ubuntu / Debian
sudo apt-get install build-essential libusb-1.0-0-dev libudev-dev

# Alpine
apk add build-base libusb-dev eudev-dev

# Fedora / RHEL
sudo dnf install gcc libusb-devel systemd-devel
```

Then build:

```bash
CGO_ENABLED=1 go install github.com/rootwarp/eth-utils/go/cmd/eth-deposit-tx@latest
```

### Windows

Windows support for the Ledger signer is not tested. The local signer (development only) may work in a CGO-enabled Windows environment (e.g., MSYS2 with MinGW-w64), but this is unsupported. Mainnet use on Windows is discouraged; use an air-gapped Linux or macOS machine instead.

## Verifying the binary

Check the version and build metadata:

```bash
eth-deposit-tx --version
# Example output:
# eth-deposit-tx version 1.0.0 (commit=abc1234, built=2026-01-15T12:00:00Z)
```

For release binaries, verify the SHA256 checksum against the published `checksums.txt` (see the [GitHub releases page](https://github.com/rootwarp/eth-utils/releases)):

```bash
sha256sum eth-deposit-tx
# Compare against checksums.txt
```

## Cross-compilation

Cross-compiling with CGO is possible but requires a cross-C toolchain targeting the host platform. For most users, build natively on the target platform or use the pre-built release binaries.

Release binaries for common platforms are planned for Issue 4.6. Until then, build from source on the target machine.

## Troubleshooting

### `cgo: C compiler "gcc" not found`

Install `gcc` (or `clang`). See [Linux notes](#linux) above.

### `undefined reference to ...` during `go install`

The CGO toolchain is not finding the right libraries. On Linux, ensure `libusb-1.0-0-dev` and `libudev-dev` are installed. On macOS, re-run `xcode-select --install`.

### Binary not found after install

Ensure `$(go env GOPATH)/bin` is in your `$PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

Add the above to your shell profile (`~/.zshrc`, `~/.bashrc`) to persist it.
