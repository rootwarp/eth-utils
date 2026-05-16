// Package keystore loads and decrypts EIP-2335 v4 keystore files.
// It exposes typed sentinel errors and a zeroize hook for key material.
package keystore

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"

	keystorev4 "github.com/wealdtech/go-eth2-wallet-encryptor-keystorev4"
)

// Sentinel errors. Callers use errors.Is to distinguish them.
var (
	// ErrKeystoreMissing is returned when the keystore file does not exist.
	ErrKeystoreMissing = errors.New("keystore file not found")

	// ErrKeystoreMalformed is returned when the keystore file cannot be parsed
	// as valid EIP-2335 JSON.
	ErrKeystoreMalformed = errors.New("keystore JSON malformed")

	// ErrKeystoreVersion is returned when the version field is not 4.
	ErrKeystoreVersion = errors.New("keystore version must be 4")

	// ErrWrongPassphrase is returned when decryption fails due to an incorrect
	// passphrase (checksum mismatch from the wealdtech encryptor).
	ErrWrongPassphrase = errors.New("wrong passphrase")

	// ErrEnvVarEmpty is returned by NewEnvSource when the named environment
	// variable is unset or empty. This maps to exit code 2 (user error).
	ErrEnvVarEmpty = errors.New("passphrase environment variable is unset or empty")
)

// Key holds the decrypted key material returned by a KeyLoader.
// Callers must call Zeroize after use; the garbage collector does not
// clear key material.
type Key struct {
	// Secret is the raw 32-byte BLS signing secret. Zeroize after use.
	Secret []byte

	// PubkeyHex is the 96-character lowercase hex public key declared in the
	// keystore JSON, without a 0x prefix.
	PubkeyHex string
}

// Zeroize overwrites every byte of Secret with 0x00.
// This must be called explicitly; Go's GC does not zero memory.
func (k *Key) Zeroize() {
	for i := range k.Secret {
		k.Secret[i] = 0x00
	}
}

// PassphraseSource abstracts where the passphrase comes from so the loader
// can be tested without a TTY or a live environment variable.
type PassphraseSource interface {
	// Read returns the passphrase bytes. The loader will zeroize the slice
	// immediately after decryption. Implementations must not retain the
	// returned slice.
	Read() ([]byte, error)
}

// KeyLoader loads and decrypts an EIP-2335 v4 keystore file.
type KeyLoader interface {
	// Load reads and decrypts the keystore at path using the passphrase
	// obtained from pw. The returned Key.Secret must be zeroized by the
	// caller via Key.Zeroize.
	Load(ctx context.Context, path string, pw PassphraseSource) (Key, error)
}

// keystoreEnvelope is the top-level structure of an EIP-2335 v4 keystore JSON.
type keystoreEnvelope struct {
	Crypto  map[string]any `json:"crypto"`
	Pubkey  string         `json:"pubkey"`
	Version int            `json:"version"`
	UUID    string         `json:"uuid"`
	Path    string         `json:"path"`
}

// loader is the concrete implementation of KeyLoader.
type loader struct{}

// NewLoader returns a KeyLoader that reads EIP-2335 v4 keystore files.
func NewLoader() KeyLoader {
	return &loader{}
}

// Load reads and decrypts the keystore at path.
//
// Error mapping:
//   - file not found            → ErrKeystoreMissing
//   - invalid JSON / schema     → ErrKeystoreMalformed
//   - version field != 4        → ErrKeystoreVersion
//   - wrong passphrase          → ErrWrongPassphrase
func (l *loader) Load(_ context.Context, path string, pw PassphraseSource) (Key, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return Key{}, fmt.Errorf("%w: %s", ErrKeystoreMissing, path)
		}
		return Key{}, fmt.Errorf("%w: read %s: %v", ErrKeystoreMissing, path, err)
	}

	var envelope keystoreEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return Key{}, fmt.Errorf("%w: %s: %v", ErrKeystoreMalformed, path, err)
	}

	// Validate required fields exist at the JSON level.
	if envelope.Crypto == nil {
		return Key{}, fmt.Errorf("%w: %s: missing crypto field", ErrKeystoreMalformed, path)
	}

	if envelope.Version != 4 {
		return Key{}, fmt.Errorf("%w: %s: got %d", ErrKeystoreVersion, path, envelope.Version)
	}

	// Source the passphrase.
	passBytes, err := pw.Read()
	if err != nil {
		return Key{}, fmt.Errorf("passphrase source: %w", err)
	}

	// Decrypt. The wealdtech API takes a string. We convert from []byte, then
	// zeroize the original slice. The string copy itself cannot be zeroed (Go
	// strings are immutable), so it will persist until GC — this is unavoidable.
	passString := string(passBytes)
	zeroizeBytes(passBytes)

	enc := keystorev4.New()
	secret, err := enc.Decrypt(envelope.Crypto, passString)
	if err != nil {
		return Key{}, fmt.Errorf("%w: %v", ErrWrongPassphrase, err)
	}

	pubkeyHex := strings.ToLower(strings.TrimPrefix(envelope.Pubkey, "0x"))

	return Key{
		Secret:    secret,
		PubkeyHex: pubkeyHex,
	}, nil
}

// zeroizeBytes overwrites every byte of b with 0x00.
func zeroizeBytes(b []byte) {
	for i := range b {
		b[i] = 0x00
	}
}
