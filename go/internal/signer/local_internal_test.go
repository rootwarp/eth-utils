package signer

import (
	"encoding/hex"
	"testing"

	gethcrypto "github.com/ethereum/go-ethereum/crypto"
)

func TestLocalSigner_Close_ZeroizesKey(t *testing.T) {
	priv, err := gethcrypto.GenerateKey()
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	keyHex := hex.EncodeToString(gethcrypto.FromECDSA(priv))
	s, err := NewLocalSignerFromHex(keyHex)
	if err != nil {
		t.Fatalf("NewLocalSignerFromHex: %v", err)
	}

	if err := s.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	for i, b := range s.key {
		if b != 0 {
			t.Errorf("key[%d] = 0x%02x after Close, want 0x00", i, b)
		}
	}
}
