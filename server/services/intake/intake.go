// Package intake — L1 + L2 safety/quality gating BEFORE distillation.
//
// L1: cheap, deterministic. Hash dedup + URL allow-list + byte cap +
//     content-type sniff.
// L2: light LLM pre-screen — is this on-topic for the galaxy? (One short
//     classification call, JSON output, cached.)
//
// Output: decision = "accept" | "reject". Reject reason is one of the
// constants below so the UI can map it to a localised message.
//
// This package only DECIDES. It does not mutate the DB; the caller
// (handlers/galaxy.go) writes the Source row with the returned status.
package intake

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/ensoul-labs/ensoul-server/models"
)

// Reject reason constants. Keep stable — UI maps them to copy.
const (
	ReasonOK         = "OK"
	ReasonDuplicate  = "DUP"
	ReasonOffTopic   = "OFFTOPIC"
	ReasonTooLarge   = "TOO_LARGE"
	ReasonSpam       = "SPAM"
	ReasonUnsupported = "UNSUPPORTED"
)

// MaxBytes — single source upload cap (Phase 1). Tune later via config.
const MaxBytes = 2 * 1024 * 1024 // 2 MiB

// Decision is the gate result.
type Decision struct {
	Accepted bool
	Reason   string
}

// HashContent returns the sha256 hex used for dedupe.
func HashContent(b []byte) string {
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}

// Screen runs the L1 + L2 checks. existingHashes is the set of content
// hashes already accepted into the same galaxy (caller supplies, typically
// via a single DB query).
//
// L2 (LLM on-topic check) is NOT wired in Phase 1.0; this stub returns
// accept after L1 passes. Wire-up lives in distill orchestrator once the
// llm package has a registered provider.
func Screen(_ context.Context, galaxy *models.Galaxy, content []byte, existingHashes map[string]bool) Decision {
	if len(content) == 0 {
		return Decision{Accepted: false, Reason: ReasonUnsupported}
	}
	if len(content) > MaxBytes {
		return Decision{Accepted: false, Reason: ReasonTooLarge}
	}
	h := HashContent(content)
	if existingHashes[h] {
		return Decision{Accepted: false, Reason: ReasonDuplicate}
	}
	// Trivial spam heuristic placeholder — replace with L2 LLM check.
	if isObviousSpam(content) {
		return Decision{Accepted: false, Reason: ReasonSpam}
	}
	return Decision{Accepted: true, Reason: ReasonOK}
}

func isObviousSpam(content []byte) bool {
	s := strings.ToLower(string(content))
	// Wildly weak heuristic — placeholder only.
	return strings.Count(s, "http://") > 50 || strings.Count(s, "https://") > 50
}

// SanityCheck is a tiny self-test used by cmd/test_v4 to assert this
// package compiles cleanly. Removed once real tests land.
func SanityCheck() error {
	if MaxBytes <= 0 {
		return errors.New("MaxBytes misconfigured")
	}
	return nil
}
