// Package merkle — minimal SHA-256 Merkle tree for V4 Epoch roll-ups.
//
// Design notes:
//   - Leaves are 32-byte SHA-256 hashes (computed by caller from canonical
//     atom serialisation).
//   - Internal nodes: H(left || right). When a level has an odd count, the
//     last leaf/node is paired with itself (Bitcoin-style) — this keeps the
//     proof generation trivial and matches what most on-chain verifiers
//     expect. Document this choice in the contract.
//   - Empty input → all-zero root, depth 0. Caller decides whether to skip
//     the on-chain write for empty epochs.
package merkle

import (
	"crypto/sha256"
	"encoding/hex"
)

// Hash is a 32-byte SHA-256 digest.
type Hash [32]byte

// HashHex returns the lower-case hex string for h.
func (h Hash) HashHex() string { return hex.EncodeToString(h[:]) }

// LeafFromBytes computes sha256(b). Use a canonical serialisation upstream.
func LeafFromBytes(b []byte) Hash {
	return sha256.Sum256(b)
}

// LeafFromHex parses a hex string (with or without 0x prefix) into a Hash.
// Returns ok=false on malformed input.
func LeafFromHex(s string) (Hash, bool) {
	if len(s) >= 2 && (s[:2] == "0x" || s[:2] == "0X") {
		s = s[2:]
	}
	if len(s) != 64 {
		return Hash{}, false
	}
	b, err := hex.DecodeString(s)
	if err != nil {
		return Hash{}, false
	}
	var h Hash
	copy(h[:], b)
	return h, true
}

// Root computes the Merkle root over leaves. Returns the all-zero hash for
// an empty input.
func Root(leaves []Hash) Hash {
	if len(leaves) == 0 {
		return Hash{}
	}
	level := make([]Hash, len(leaves))
	copy(level, leaves)

	for len(level) > 1 {
		// Pair up; duplicate last if odd.
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1])
		}
		next := make([]Hash, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			next[i/2] = pair(level[i], level[i+1])
		}
		level = next
	}
	return level[0]
}

// Proof returns the sibling path needed to verify leaves[index] against the
// root computed by Root(). Each entry is the sibling hash at that level,
// in order from leaf upward.
func Proof(leaves []Hash, index int) []Hash {
	if index < 0 || index >= len(leaves) {
		return nil
	}
	level := make([]Hash, len(leaves))
	copy(level, leaves)
	idx := index
	var path []Hash

	for len(level) > 1 {
		if len(level)%2 == 1 {
			level = append(level, level[len(level)-1])
		}
		var sibling Hash
		if idx%2 == 0 {
			sibling = level[idx+1]
		} else {
			sibling = level[idx-1]
		}
		path = append(path, sibling)

		next := make([]Hash, len(level)/2)
		for i := 0; i < len(level); i += 2 {
			next[i/2] = pair(level[i], level[i+1])
		}
		level = next
		idx /= 2
	}
	return path
}

// Verify checks that hashing leaf with the supplied sibling path reproduces
// root, assuming index is the leaf's original position.
func Verify(leaf Hash, path []Hash, index int, root Hash) bool {
	cur := leaf
	idx := index
	for _, sib := range path {
		if idx%2 == 0 {
			cur = pair(cur, sib)
		} else {
			cur = pair(sib, cur)
		}
		idx /= 2
	}
	return cur == root
}

func pair(l, r Hash) Hash {
	var buf [64]byte
	copy(buf[:32], l[:])
	copy(buf[32:], r[:])
	return sha256.Sum256(buf[:])
}
