package signer

import "github.com/rootwarp/eth-utils/go/internal/tx"

// SignedTx is a signed EIP-1559 deposit transaction ready for broadcast.
// The fields mirror UnsignedTx but include the signature (r, s, v) and
// the RLP-encoded raw bytes that can be sent via eth_sendRawTransaction.
type SignedTx struct {
	// Unsigned is the original unsigned transaction this signature applies to.
	Unsigned tx.UnsignedTx `json:"unsigned"`
	// From is the recovered sender address (0x-prefixed hex), derived from the signature.
	From string `json:"from"`
	// Hash is the transaction hash (Keccak-256 of the signed RLP), 0x-prefixed hex.
	Hash string `json:"hash"`
	// R is the signature R value, 0x-prefixed hex.
	R string `json:"r"`
	// S is the signature S value, 0x-prefixed hex.
	S string `json:"s"`
	// V is the signature V value. For EIP-1559 (type-2) transactions this is
	// the y-parity bit encoded as a decimal string: "0" or "1".
	V string `json:"v"`
	// RawRLP is the 0x-prefixed hex RLP encoding of the signed transaction,
	// directly usable with eth_sendRawTransaction.
	RawRLP string `json:"rawRLP"`
}
