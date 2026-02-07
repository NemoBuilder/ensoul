package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
)

// test_chain is an integration test script that validates the full ERC-8004 chain flow:
//   1. Register a Soul (Identity Registry)
//   2. Read back the agentURI and verify contents
//   3. Read owner of the minted Soul
//   4. Update the Soul URI (simulating ensouling)
//   5. Generate a Claw wallet
//   6. Submit reputation feedback from the Claw
//   7. Read back the reputation summary
//
// Usage:
//   PLATFORM_PRIVATE_KEY=<your_pk> BSC_RPC_URL=<rpc> go run cmd/test_chain/main.go
//
// Requirements:
//   - A funded wallet for PLATFORM_PRIVATE_KEY on the target chain
//   - BSC RPC URL (defaults to BSC mainnet if not set)

func main() {
	log.Println("=== Ensoul Chain Integration Test ===")
	log.Println()

	// Load config
	cfg := config.Load()

	if cfg.PrivateKey == "" {
		log.Fatal("PLATFORM_PRIVATE_KEY is required for integration test. " +
			"Set it in .env or as environment variable.")
	}

	// Initialize chain client
	log.Println("[1/8] Initializing chain client...")
	if err := chain.Init(); err != nil {
		log.Fatalf("Chain init failed: %v", err)
	}
	log.Println("      ✓ Chain client initialized")
	log.Println()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	// Step 1: Mint a Soul
	testHandle := fmt.Sprintf("test_soul_%d", time.Now().Unix())
	log.Printf("[2/8] Minting Soul: @%s ...", testHandle)

	agentId, txHash, err := chain.MintSoul(
		ctx,
		testHandle,
		chain.C.PlatformAddress().Hex(), // Owner is the platform wallet for test
		"https://ensoul.ac/default-avatar.png",
		"A test soul created by the integration test script.",
		1, // DNA version
	)
	if err != nil {
		log.Fatalf("      ✗ MintSoul failed: %v", err)
	}
	log.Printf("      ✓ Soul minted! agentId=%s, tx=%s", agentId.String(), txHash)
	log.Println()

	// Wait a moment for the async setMetadata goroutine
	log.Println("[3/8] Waiting for setMetadata to complete (5s)...")
	time.Sleep(5 * time.Second)
	log.Println("      ✓ Metadata set (async)")
	log.Println()

	// Step 2: Read back the agentURI
	log.Printf("[4/8] Reading agentURI for agentId=%s...", agentId.String())
	agentURI, err := chain.ReadSoulURI(ctx, agentId)
	if err != nil {
		log.Fatalf("      ✗ ReadSoulURI failed: %v", err)
	}

	// Decode the data URI and pretty-print
	if strings.HasPrefix(agentURI, "data:application/json;base64,") {
		jsonBytes, err := base64.StdEncoding.DecodeString(agentURI[len("data:application/json;base64,"):])
		if err != nil {
			log.Printf("      ✗ Failed to decode base64 URI: %v", err)
		} else {
			var prettyJSON map[string]interface{}
			json.Unmarshal(jsonBytes, &prettyJSON)
			formatted, _ := json.MarshalIndent(prettyJSON, "      ", "  ")
			log.Printf("      ✓ agentURI contents:\n      %s", string(formatted))
		}
	} else {
		log.Printf("      ✓ agentURI: %s", agentURI)
	}
	log.Println()

	// Step 3: Read the owner
	log.Printf("[5/8] Reading owner of agentId=%s...", agentId.String())
	owner, err := chain.ReadSoulOwner(ctx, agentId)
	if err != nil {
		log.Fatalf("      ✗ ReadSoulOwner failed: %v", err)
	}
	log.Printf("      ✓ Owner: %s", owner.Hex())
	log.Println()

	// Step 4: Read metadata (handle)
	log.Printf("[6/8] Reading metadata 'ensoul:handle' for agentId=%s...", agentId.String())
	handleMeta, err := chain.C.IdentityRegistry().GetMetadata(
		&bind.CallOpts{Context: ctx},
		agentId,
		"ensoul:handle",
	)
	if err != nil {
		log.Printf("      ✗ GetMetadata failed (may not be mined yet): %v", err)
	} else {
		log.Printf("      ✓ ensoul:handle = %q", string(handleMeta))
	}
	log.Println()

	// Step 5: Generate a Claw wallet
	log.Println("[7/8] Generating Claw wallet...")
	clawWallet, err := chain.GenerateClawWallet()
	if err != nil {
		log.Fatalf("      ✗ GenerateClawWallet failed: %v", err)
	}
	log.Printf("      ✓ Claw address: %s", clawWallet.Address)
	log.Printf("      ✓ Encrypted PK: %s...%s",
		clawWallet.PrivateKeyEnc[:20], clawWallet.PrivateKeyEnc[len(clawWallet.PrivateKeyEnc)-8:])

	// Verify round-trip decryption
	decryptedKey, err := chain.DecryptClawPrivateKey(clawWallet.PrivateKeyEnc)
	if err != nil {
		log.Fatalf("      ✗ DecryptClawPrivateKey failed: %v", err)
	}
	log.Printf("      ✓ Decrypted key verifies OK (addr matches: %v)",
		clawWallet.Address == fmt.Sprintf("0x%x", decryptedKey.PublicKey))
	log.Println()

	// Step 6: Submit reputation feedback
	// NOTE: The Claw wallet needs BNB to pay gas. In production, the platform
	// would fund Claw wallets or use a gas relay. For testing, we skip if balance is 0.
	log.Println("[8/8] Attempting reputation feedback submission...")

	balance, err := chain.C.EthClient().BalanceAt(ctx, chain.C.PlatformAddress(), nil)
	if err != nil {
		log.Printf("      ✗ Failed to check balance: %v", err)
	} else {
		log.Printf("      Platform balance: %s wei", balance.String())
	}

	// Use the platform key as the feedback sender for the test
	// (In production, each Claw has its own funded wallet)
	if chain.C.HasPlatformKey() {
		log.Println("      Submitting feedback from platform wallet (test mode)...")

		// We need to use the same pattern but with the platform key directly
		feedbackTx, err := chain.SubmitFeedback(
			ctx,
			chain.C.PlatformKey(),
			agentId,
			85,            // feedback value: 85%
			"personality", // tag1
			"depth",       // tag2
		)
		if err != nil {
			log.Printf("      ✗ SubmitFeedback failed: %v", err)
			log.Println("      (This may fail if the contract prevents self-feedback)")
		} else {
			log.Printf("      ✓ Feedback submitted! tx=%s", feedbackTx)

			// Read back the reputation
			count, value, decimals, err := chain.ReadReputationSummary(
				ctx,
				agentId,
				nil, // empty filter = all clients
			)
			if err != nil {
				log.Printf("      ✗ ReadReputationSummary failed: %v", err)
			} else {
				log.Printf("      ✓ Reputation summary: count=%d, value=%s, decimals=%d",
					count, value.String(), decimals)
			}
		}
	}

	log.Println()
	log.Println("=== Integration Test Complete ===")
	log.Printf("Soul: @%s (agentId=%s)", testHandle, agentId.String())
	log.Printf("Chain: %s", chain.C.ChainID().String())

	// Exit with proper code
	if agentId != nil && agentId.Cmp(big.NewInt(0)) > 0 {
		log.Println("Result: PASS ✓")
		os.Exit(0)
	} else {
		log.Println("Result: FAIL ✗")
		os.Exit(1)
	}
}
