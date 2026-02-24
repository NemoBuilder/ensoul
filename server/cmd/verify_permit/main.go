package main

import (
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	// ── Data from the API response ──────────────────────────────
	handleHash := common.HexToHash("0xf4fd82c00ce9b21f94c879356a23b67fd2504dce4734a55e630ea3f0a26fd8cd")
	priceWei, _ := new(big.Int).SetString("100000000000000000", 10)
	deadline := int64(1771860344)
	nonce := uint64(1771858544519358800)
	sigHex := "5b2edbee1ef6f4f8f5148fa60d2eab7a0d5008c49785d3ec03dcacfa818655173e5067443c97fccadc39123cd374eb9730fc4fcee7fc6eb695b8f47f14a17e1a1c"

	// ── Known addresses ─────────────────────────────────────────
	minterAddr := common.HexToAddress("0xc5aE375Dfd8042e9345F1bB8e3b039b6d4690023")
	platformAddr := common.HexToAddress("0xAEF83196022a4301a261C03FD3335a533e0Ad18d")
	chainID := big.NewInt(56)

	// ── What user wallet was used? ──────────────────────────────
	// We need to figure out the user wallet address that was passed.
	// Try the test wallet and also a few variations.

	fmt.Println("========================================")
	fmt.Println("  Permit Signature Verification")
	fmt.Println("========================================")
	fmt.Println()

	// First, verify the handleHash
	expectedHandleHash := crypto.Keccak256Hash([]byte("framer_x"))
	fmt.Println("Expected handleHash: ", expectedHandleHash.Hex())
	fmt.Println("API handleHash:      ", handleHash.Hex())
	if handleHash == expectedHandleHash {
		fmt.Println("✅ handleHash matches")
	} else {
		fmt.Println("❌ handleHash MISMATCH!")
	}
	fmt.Println()

	// Decode signature
	sig := common.FromHex(sigHex)
	fmt.Println("Signature length:", len(sig), "(expect 65)")

	// Adjust V for recovery (27/28 -> 0/1)
	sigCopy := make([]byte, 65)
	copy(sigCopy, sig)
	if sigCopy[64] >= 27 {
		sigCopy[64] -= 27
	}

	// Try recovering with different user addresses to find which one was used
	testAddrs := []common.Address{
		common.HexToAddress("0x603C05922C42D0703B4D8678d9595D23A358050a"), // test wallet
	}

	// Also let's just try recovering and see what address comes out
	// by using a known user address first
	for _, userAddr := range testAddrs {
		fmt.Printf("\n--- Trying userAddr: %s ---\n", userAddr.Hex())

		packed := []byte{}
		packed = append(packed, handleHash.Bytes()...)                                             // bytes32: 32
		packed = append(packed, common.LeftPadBytes(priceWei.Bytes(), 32)...)                      // uint256: 32
		packed = append(packed, userAddr.Bytes()...)                                               // address: 20
		packed = append(packed, common.LeftPadBytes(big.NewInt(deadline).Bytes(), 32)...)          // uint256: 32
		packed = append(packed, common.LeftPadBytes(new(big.Int).SetUint64(nonce).Bytes(), 32)...) // uint256: 32
		packed = append(packed, common.LeftPadBytes(chainID.Bytes(), 32)...)                       // uint256: 32
		packed = append(packed, minterAddr.Bytes()...)                                             // address: 20
		fmt.Println("Packed length:", len(packed), "(expect 200)")

		messageHash := crypto.Keccak256Hash(packed)
		fmt.Println("MessageHash:", messageHash.Hex())

		prefix := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n32"))
		prefixed := crypto.Keccak256Hash(append(prefix, messageHash.Bytes()...))
		fmt.Println("EthSignedHash:", prefixed.Hex())

		recoveredPub, err := crypto.Ecrecover(prefixed.Bytes(), sigCopy)
		if err != nil {
			fmt.Println("❌ Ecrecover failed:", err)
			continue
		}
		pubKey, err := crypto.UnmarshalPubkey(recoveredPub)
		if err != nil {
			fmt.Println("❌ UnmarshalPubkey failed:", err)
			continue
		}
		recovered := crypto.PubkeyToAddress(*pubKey)
		fmt.Println("Recovered signer:", recovered.Hex())
		fmt.Println("Expected signer: ", platformAddr.Hex())
		if recovered == platformAddr {
			fmt.Println("✅ Signature VALID for this userAddr!")
		} else {
			fmt.Println("❌ Signature does NOT match platform signer")
		}
	}

	// ── Brute-force: recover without knowing userAddr ───────────
	// Let's also try with the user address being embedded in the signature
	// by just trying to recover with a WRONG userAddr to see what signer we get
	fmt.Println("\n\n========================================")
	fmt.Println("  Also try: what if backend used 32-byte padded address?")
	fmt.Println("========================================")

	for _, userAddr := range testAddrs {
		fmt.Printf("\n--- 32-byte padded userAddr: %s ---\n", userAddr.Hex())

		packed := []byte{}
		packed = append(packed, handleHash.Bytes()...)                                             // bytes32: 32
		packed = append(packed, common.LeftPadBytes(priceWei.Bytes(), 32)...)                      // uint256: 32
		packed = append(packed, common.LeftPadBytes(userAddr.Bytes(), 32)...)                      // address: 32 (OLD BUG)
		packed = append(packed, common.LeftPadBytes(big.NewInt(deadline).Bytes(), 32)...)          // uint256: 32
		packed = append(packed, common.LeftPadBytes(new(big.Int).SetUint64(nonce).Bytes(), 32)...) // uint256: 32
		packed = append(packed, common.LeftPadBytes(chainID.Bytes(), 32)...)                       // uint256: 32
		packed = append(packed, common.LeftPadBytes(minterAddr.Bytes(), 32)...)                    // address: 32 (OLD BUG)
		fmt.Println("Packed length:", len(packed), "(would be 224 with old bug)")

		messageHash := crypto.Keccak256Hash(packed)
		prefix := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n32"))
		prefixed := crypto.Keccak256Hash(append(prefix, messageHash.Bytes()...))

		recoveredPub, err := crypto.Ecrecover(prefixed.Bytes(), sigCopy)
		if err != nil {
			fmt.Println("❌ Ecrecover failed:", err)
			continue
		}
		pubKey, _ := crypto.UnmarshalPubkey(recoveredPub)
		recovered := crypto.PubkeyToAddress(*pubKey)
		fmt.Println("Recovered signer:", recovered.Hex())
		if recovered == platformAddr {
			fmt.Println("⚠️  OLD BUG STILL PRESENT — backend binary was NOT recompiled!")
		} else {
			fmt.Println("(not matching, expected)")
		}
	}
}
