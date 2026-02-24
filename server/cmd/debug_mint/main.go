package main

import (
	"context"
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
	rpcURL         = "https://bsc-dataseed.bnbchain.org"
	minterV2Addr   = "0xc5aE375Dfd8042e9345F1bB8e3b039b6d4690023"
	registryAddr   = "0x8004A169FB4a3325136EB29fA0ceB6D2e539c432"
	platformKeyHex = "c6335b98269a0574746ed6b613fe3d0a99da7c47a0e4cb9be8c120e5d3ae0173"
	testUserAddr   = "0xe51F749283c3fb21eF602b7AAAeb2cF73df210F2" // deployer as test user
)

func main() {
	ctx := context.Background()
	client, err := ethclient.Dial(rpcURL)
	if err != nil {
		fmt.Println("Failed to connect:", err)
		os.Exit(1)
	}
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	if err != nil {
		fmt.Println("Failed to get chainID:", err)
		os.Exit(1)
	}
	fmt.Println("Chain ID:", chainID)

	minter := common.HexToAddress(minterV2Addr)
	registry := common.HexToAddress(registryAddr)

	// ============================================================
	// 1. Read MinterV2 state
	// ============================================================
	fmt.Println("\n=== EnsoulMinterV2 State ===")

	// Read trustedSigner
	trustedSignerSig := crypto.Keccak256([]byte("trustedSigner()"))[:4]
	result, err := client.CallContract(ctx, ethereum.CallMsg{To: &minter, Data: trustedSignerSig}, nil)
	if err != nil {
		fmt.Println("trustedSigner() error:", err)
	} else {
		fmt.Println("trustedSigner:", common.BytesToAddress(result).Hex())
	}

	// Read registry
	registrySig := crypto.Keccak256([]byte("registry()"))[:4]
	result, err = client.CallContract(ctx, ethereum.CallMsg{To: &minter, Data: registrySig}, nil)
	if err != nil {
		fmt.Println("registry() error:", err)
	} else {
		fmt.Println("registry:", common.BytesToAddress(result).Hex())
	}

	// Read treasury
	treasurySig := crypto.Keccak256([]byte("treasury()"))[:4]
	result, err = client.CallContract(ctx, ethereum.CallMsg{To: &minter, Data: treasurySig}, nil)
	if err != nil {
		fmt.Println("treasury() error:", err)
	} else {
		fmt.Println("treasury:", common.BytesToAddress(result).Hex())
	}

	// Read paused
	pausedSig := crypto.Keccak256([]byte("paused()"))[:4]
	result, err = client.CallContract(ctx, ethereum.CallMsg{To: &minter, Data: pausedSig}, nil)
	if err != nil {
		fmt.Println("paused() error:", err)
	} else {
		isPaused := new(big.Int).SetBytes(result).Int64() != 0
		fmt.Println("paused:", isPaused)
	}

	// Read owner
	ownerSig := crypto.Keccak256([]byte("owner()"))[:4]
	result, err = client.CallContract(ctx, ethereum.CallMsg{To: &minter, Data: ownerSig}, nil)
	if err != nil {
		fmt.Println("owner() error:", err)
	} else {
		fmt.Println("owner:", common.BytesToAddress(result).Hex())
	}

	// ============================================================
	// 2. Read IdentityRegistry state
	// ============================================================
	fmt.Println("\n=== IdentityRegistry State ===")

	// Read owner of registry
	result, err = client.CallContract(ctx, ethereum.CallMsg{To: &registry, Data: ownerSig}, nil)
	if err != nil {
		fmt.Println("registry owner() error:", err)
	} else {
		fmt.Println("registry owner:", common.BytesToAddress(result).Hex())
	}

	// Read getVersion
	versionSig := crypto.Keccak256([]byte("getVersion()"))[:4]
	result, err = client.CallContract(ctx, ethereum.CallMsg{To: &registry, Data: versionSig}, nil)
	if err != nil {
		fmt.Println("getVersion() error:", err)
	} else {
		// ABI decode string
		if len(result) >= 64 {
			offset := new(big.Int).SetBytes(result[:32]).Int64()
			length := new(big.Int).SetBytes(result[offset : offset+32]).Int64()
			version := string(result[offset+32 : offset+32+length])
			fmt.Println("registry version:", version)
		}
	}

	// ============================================================
	// 3. Try calling registry.register() directly as MinterV2
	//    via eth_call to see if it reverts
	// ============================================================
	fmt.Println("\n=== Simulate registry.register() from MinterV2 ===")

	// Build register(string) call data
	registerABI, _ := abi.JSON(strings.NewReader(`[{"type":"function","name":"register","inputs":[{"name":"agentURI","type":"string"}],"outputs":[{"name":"agentId","type":"uint256"}],"stateMutability":"nonpayable"}]`))
	registerData, _ := registerABI.Pack("register", "ensoul://soul/test_debug")

	// Simulate calling register from the MinterV2 address
	result, err = client.CallContract(ctx, ethereum.CallMsg{
		From: minter, // simulate call from MinterV2
		To:   &registry,
		Data: registerData,
	}, nil)
	if err != nil {
		fmt.Println("❌ register() from MinterV2 REVERTED:", err)
	} else {
		agentId := new(big.Int).SetBytes(result)
		fmt.Println("✅ register() would succeed, agentId:", agentId)
	}

	// Also try calling register from a normal EOA
	userAddr := common.HexToAddress(testUserAddr)
	result, err = client.CallContract(ctx, ethereum.CallMsg{
		From: userAddr,
		To:   &registry,
		Data: registerData,
	}, nil)
	if err != nil {
		fmt.Println("❌ register() from EOA REVERTED:", err)
	} else {
		agentId := new(big.Int).SetBytes(result)
		fmt.Println("✅ register() from EOA would succeed, agentId:", agentId)
	}

	// ============================================================
	// 4. Simulate full mint() call
	// ============================================================
	fmt.Println("\n=== Simulate full mint() ===")

	platformKey, _ := crypto.HexToECDSA(platformKeyHex)
	platformAddr := crypto.PubkeyToAddress(platformKey.PublicKey)
	fmt.Println("Platform addr:", platformAddr.Hex())

	handle := "debug_test_handle_12345"
	handleHash := crypto.Keccak256Hash([]byte(strings.ToLower(handle)))
	priceWei := big.NewInt(10000000000000000) // 0.01 BNB
	deadline := int64(1772000000)             // 2026-02-25, actually in the future
	nonce := uint64(99999)

	// Build message hash same as contract
	messageHash := crypto.Keccak256Hash(
		handleHash.Bytes(),
		common.LeftPadBytes(priceWei.Bytes(), 32),
		userAddr.Bytes(),
		common.LeftPadBytes(big.NewInt(deadline).Bytes(), 32),
		common.LeftPadBytes(new(big.Int).SetUint64(nonce).Bytes(), 32),
		common.LeftPadBytes(chainID.Bytes(), 32),
		minter.Bytes(),
	)

	// EIP-191 prefix
	prefixed := crypto.Keccak256Hash(
		[]byte(fmt.Sprintf("\x19Ethereum Signed Message:\n32")),
		messageHash.Bytes(),
	)

	sig, err := crypto.Sign(prefixed.Bytes(), platformKey)
	if err != nil {
		fmt.Println("Sign error:", err)
		os.Exit(1)
	}
	if sig[64] < 27 {
		sig[64] += 27
	}

	// Build mint() ABI call
	mintABI, _ := abi.JSON(strings.NewReader(`[{"type":"function","name":"mint","inputs":[{"name":"agentURI","type":"string"},{"name":"handleHash","type":"bytes32"},{"name":"price","type":"uint256"},{"name":"deadline","type":"uint256"},{"name":"nonce","type":"uint256"},{"name":"signature","type":"bytes"}],"outputs":[{"name":"agentId","type":"uint256"}],"stateMutability":"payable"}]`))

	agentURI := "ensoul://soul/" + strings.ToLower(handle)
	mintData, err := mintABI.Pack("mint", agentURI, handleHash, priceWei, big.NewInt(deadline), new(big.Int).SetUint64(nonce), sig)
	if err != nil {
		fmt.Println("Pack mint error:", err)
		os.Exit(1)
	}

	// Simulate the mint call
	result, err = client.CallContract(ctx, ethereum.CallMsg{
		From:  userAddr,
		To:    &minter,
		Value: priceWei,
		Data:  mintData,
	}, nil)
	if err != nil {
		fmt.Println("❌ mint() REVERTED:", err)
		fmt.Println("   This is the root cause of the MetaMask failure!")
		fmt.Println("\n   Possible reasons:")
		fmt.Println("   1. Signature mismatch (trustedSigner doesn't match)")
		fmt.Println("   2. Deadline expired")
		fmt.Println("   3. Nonce already used")
		fmt.Println("   4. Handle already minted")
		fmt.Println("   5. registry.register() fails from MinterV2 context")
		fmt.Println("   6. safeTransferFrom fails")
		fmt.Println("   7. treasury.call fails")
	} else {
		agentId := new(big.Int).SetBytes(result)
		fmt.Println("✅ mint() would succeed! agentId:", agentId)
	}
}
