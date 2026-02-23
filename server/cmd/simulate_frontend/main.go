package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	rpcURL       = "https://bsc-dataseed.binance.org/"
	minterV2Addr = "0x76D5361D768Cf9AA9b3088ce9C2760d6Cd76466B"
)

func main() {
	ctx := context.Background()

	// ── Exact data from API response ────────────────────────────
	handleHashHex := "0xf4fd82c00ce9b21f94c879356a23b67fd2504dce4734a55e630ea3f0a26fd8cd"
	priceStr := "100000000000000000"
	deadline := int64(1771860838)
	nonceStr := "1771859038619374100"
	sigHex := "0x638eb228b0dc90c5e196c92ff13fa60d42b8e7c2922c0f7018df203d0983278d5dd63353b39187aa2a238df91d33180f24a9bf085d281e63d4e97a5565d68e951b"
	priceWeiStr := "100000000000000000"

	// The user's wallet address — we need to figure this out
	// Let's try the test wallet first, and also try a few others
	callerAddr := common.HexToAddress("0x603C05922C42D0703B4D8678d9595D23A358050a")

	fmt.Println("========================================")
	fmt.Println("  Simulate Frontend Mint Call")
	fmt.Println("========================================")

	client, err := ethclient.Dial(rpcURL)
	fatal("connect", err)
	defer client.Close()

	chainID, _ := client.ChainID(ctx)
	fmt.Println("Chain ID:", chainID)

	minter := common.HexToAddress(minterV2Addr)

	// ── 1. First check: is framer_x already minted? ─────────────
	handleHash := common.HexToHash(handleHashHex)
	isHandleMintedABI, _ := abi.JSON(strings.NewReader(`[{"type":"function","name":"isHandleMinted","inputs":[{"name":"handleHash","type":"bytes32"}],"outputs":[{"name":"","type":"bool"}]}]`))
	callData, _ := isHandleMintedABI.Pack("isHandleMinted", handleHash)
	result, err := client.CallContract(ctx, ethereum.CallMsg{To: &minter, Data: callData}, nil)
	if err == nil && len(result) >= 32 && result[31] == 1 {
		fmt.Println("❌ Handle 'framer_x' is ALREADY MINTED! This is why the frontend fails!")
		fmt.Println("   The test script already minted this handle as Agent ID 6613.")
		fmt.Println("   Try minting a DIFFERENT handle from the UI.")
		os.Exit(0)
	}
	fmt.Println("✅ Handle not yet minted")

	// ── 2. Verify permit signature ─────────────────────────────
	fmt.Println("\n--- Verifying Permit Signature ---")

	priceWei, _ := new(big.Int).SetString(priceStr, 10)
	nonce, _ := new(big.Int).SetString(nonceStr, 10)

	// Verify: recover signer from the permit signature
	packed := []byte{}
	packed = append(packed, handleHash.Bytes()...)                                 // bytes32: 32
	packed = append(packed, common.LeftPadBytes(priceWei.Bytes(), 32)...)          // uint256: 32
	packed = append(packed, callerAddr.Bytes()...)                                 // address: 20
	packed = append(packed, common.LeftPadBytes(big.NewInt(deadline).Bytes(), 32)...) // uint256: 32
	packed = append(packed, common.LeftPadBytes(nonce.Bytes(), 32)...)             // uint256: 32
	packed = append(packed, common.LeftPadBytes(chainID.Bytes(), 32)...)           // uint256: 32
	packed = append(packed, minter.Bytes()...)                                     // address: 20
	fmt.Println("Packed length:", len(packed), "(expect 200)")

	messageHash := crypto.Keccak256Hash(packed)
	prefix := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n32"))
	prefixed := crypto.Keccak256Hash(append(prefix, messageHash.Bytes()...))

	sig := common.FromHex(sigHex)
	sigForRecover := make([]byte, 65)
	copy(sigForRecover, sig)
	if sigForRecover[64] >= 27 {
		sigForRecover[64] -= 27
	}

	recoveredPub, err := crypto.Ecrecover(prefixed.Bytes(), sigForRecover)
	if err != nil {
		fmt.Println("❌ Ecrecover failed:", err)
	} else {
		pubKey, _ := crypto.UnmarshalPubkey(recoveredPub)
		recovered := crypto.PubkeyToAddress(*pubKey)
		platformAddr := common.HexToAddress("0xAEF83196022a4301a261C03FD3335a533e0Ad18d")
		fmt.Println("Recovered signer:", recovered.Hex())
		fmt.Println("Expected signer: ", platformAddr.Hex())
		if recovered == platformAddr {
			fmt.Println("✅ Signature is VALID")
		} else {
			fmt.Println("❌ Signature INVALID — signer mismatch")
			fmt.Println("   This means the user wallet used by the frontend is NOT", callerAddr.Hex())
			fmt.Println("   Or there is still a packing issue.")
		}
	}

	// ── 3. Build agentURI same way as frontend ─────────────────
	fmt.Println("\n--- Building agentURI (frontend style) ---")

	// The frontend builds a data:application/json;base64,... URI
	regFile := map[string]interface{}{
		"type":        "https://eips.ethereum.org/EIPS/eip-8004#registration-v1",
		"name":        "@framer_x · Ensoul",
		"description": "test",
		"image":       "https://pbs.twimg.com/profile_images/test/photo.jpg",
		"services": []map[string]string{
			{"name": "web", "endpoint": "https://ensoul.ac/soul/framer_x"},
			{"name": "chat", "endpoint": "https://ensoul.ac/soul/framer_x/chat"},
		},
		"active": true,
		"ensoul": map[string]interface{}{
			"handle":     "framer_x",
			"stage":      "embryo",
			"dnaVersion": 1,
		},
	}
	jsonBytes, _ := json.Marshal(regFile)
	base64Str := base64.StdEncoding.EncodeToString(jsonBytes)
	frontendAgentURI := "data:application/json;base64," + base64Str
	fmt.Println("Frontend agentURI length:", len(frontendAgentURI))

	// Also the simple test script URI
	simpleAgentURI := "ensoul://soul/framer_x"
	fmt.Println("Test script agentURI:", simpleAgentURI)

	// ── 4. Simulate mint() with FRONTEND agentURI ──────────────
	fmt.Println("\n--- Simulating mint() with frontend-style agentURI ---")

	mintABI, _ := abi.JSON(strings.NewReader(`[{"type":"function","name":"mint","inputs":[{"name":"agentURI","type":"string"},{"name":"handleHash","type":"bytes32"},{"name":"price","type":"uint256"},{"name":"deadline","type":"uint256"},{"name":"nonce","type":"uint256"},{"name":"signature","type":"bytes"}],"outputs":[{"name":"agentId","type":"uint256"}],"stateMutability":"payable"}]`))

	priceVal, _ := new(big.Int).SetString(priceWeiStr, 10)

	mintData, err := mintABI.Pack("mint",
		frontendAgentURI,
		handleHash,
		priceVal,
		big.NewInt(deadline),
		nonce,
		sig,
	)
	fatal("pack mint", err)

	result, err = client.CallContract(ctx, ethereum.CallMsg{
		From:  callerAddr,
		To:    &minter,
		Value: priceVal,
		Data:  mintData,
	}, nil)
	if err != nil {
		fmt.Println("❌ REVERTED:", err)
		errStr := err.Error()
		if strings.Contains(errStr, "cab64075") {
			fmt.Println("   → ExpiredPermit()")
		} else if strings.Contains(errStr, "8baa579f") {
			fmt.Println("   → InvalidSignature()")
		} else if strings.Contains(errStr, "ef43ee49") {
			fmt.Println("   → HandleAlreadyMinted()")
		} else if strings.Contains(errStr, "1fb09b80") {
			fmt.Println("   → NonceAlreadyUsed()")
		}
	} else {
		fmt.Println("✅ mint() simulation SUCCEEDED! agentId:", new(big.Int).SetBytes(result))
	}
}

func fatal(label string, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL [%s]: %v\n", label, err)
		os.Exit(1)
	}
}
