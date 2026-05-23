package merkle

import (
	"testing"
)

func TestRootStability(t *testing.T) {
	leaves := []Hash{
		LeafFromBytes([]byte("a")),
		LeafFromBytes([]byte("b")),
		LeafFromBytes([]byte("c")),
	}
	r1 := Root(leaves)
	r2 := Root(leaves)
	if r1 != r2 {
		t.Fatalf("Root is non-deterministic")
	}
	if r1 == (Hash{}) {
		t.Fatalf("Root unexpectedly zero")
	}
}

func TestProofVerify(t *testing.T) {
	leaves := []Hash{
		LeafFromBytes([]byte("alpha")),
		LeafFromBytes([]byte("beta")),
		LeafFromBytes([]byte("gamma")),
		LeafFromBytes([]byte("delta")),
		LeafFromBytes([]byte("epsilon")), // odd → triggers duplicate-last
	}
	root := Root(leaves)
	for i, leaf := range leaves {
		path := Proof(leaves, i)
		if !Verify(leaf, path, i, root) {
			t.Fatalf("verify failed for index %d", i)
		}
	}
	// Tampered leaf must NOT verify.
	bad := LeafFromBytes([]byte("tampered"))
	if Verify(bad, Proof(leaves, 0), 0, root) {
		t.Fatalf("tampered leaf incorrectly verified")
	}
}

func TestEmpty(t *testing.T) {
	if Root(nil) != (Hash{}) {
		t.Fatalf("empty root must be zero")
	}
}
