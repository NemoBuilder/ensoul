package main

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	key, _ := crypto.HexToECDSA("c6335b98269a0574746ed6b613fe3d0a99da7c47a0e4cb9be8c120e5d3ae0173")
	addr := crypto.PubkeyToAddress(key.PublicKey)
	fmt.Println("Platform (trustedSigner):", addr.Hex())

	handle := "elonmusk"
	handleHash := crypto.Keccak256Hash([]byte(handle))
	priceWei := big.NewInt(100000000000000000) // 0.1 BNB
	userAddr := common.HexToAddress("0x1234567890abcdef1234567890abcdef12345678")
	deadline := int64(1740000000)
	nonce := uint64(1234567890)
	chainID := big.NewInt(56)
	minterAddr := common.HexToAddress("0xc5aE375Dfd8042e9345F1bB8e3b039b6d4690023")

	fmt.Println("\n=== Step 1: abi.encodePacked ===")

	// Solidity abi.encodePacked types:
	// bytes32 = 32, uint256 = 32, address = 20, uint256 = 32, uint256 = 32, uint256 = 32, address = 20
	// total = 200 bytes
	packed := []byte{}
	packed = append(packed, handleHash.Bytes()...)                                             // bytes32: 32
	packed = append(packed, common.LeftPadBytes(priceWei.Bytes(), 32)...)                      // uint256: 32
	packed = append(packed, userAddr.Bytes()...)                                               // address: 20
	packed = append(packed, common.LeftPadBytes(big.NewInt(deadline).Bytes(), 32)...)          // uint256: 32
	packed = append(packed, common.LeftPadBytes(new(big.Int).SetUint64(nonce).Bytes(), 32)...) // uint256: 32
	packed = append(packed, common.LeftPadBytes(chainID.Bytes(), 32)...)                       // uint256: 32
	packed = append(packed, minterAddr.Bytes()...)                                             // address: 20
	fmt.Printf("packed length: %d bytes (expect 200)\n", len(packed))
	fmt.Printf("packed hex: 0x%s\n", hex.EncodeToString(packed))

	messageHash := crypto.Keccak256Hash(packed)
	fmt.Println("messageHash:", messageHash.Hex())

	fmt.Println("\n=== Step 2: toEthSignedMessageHash ===")

	// Method A: Go simple concat (what permit.go does)
	goPrefix := []byte("\x19Ethereum Signed Message:\n32")
	fmt.Printf("Go prefix: %q (len=%d)\n", string(goPrefix), len(goPrefix))
	goConcat := append(goPrefix, messageHash.Bytes()...)
	fmt.Printf("Go concat length: %d\n", len(goConcat))
	goHash := crypto.Keccak256Hash(goConcat)
	fmt.Println("Go ethSignedHash:", goHash.Hex())

	// Method B: Simulate Solidity assembly exactly
	// mstore(0x00, "\x19Ethereum Signed Message:\n32") — writes 32 bytes at offset 0
	// The string is 26 chars, right-padded with 6 zero bytes to fill 32-byte word
	// mstore(0x1c, messageHash) — writes 32 bytes at offset 28
	// This OVERWRITES the last 4 zero-padding bytes of the prefix!
	// keccak256(0x00, 0x3c) — hashes 60 bytes
	mem := make([]byte, 64) // enough space

	// mstore(0x00, "\x19Ethereum Signed Message:\n32")
	// In Solidity, a string literal in mstore is left-aligned (big-endian) in a 32-byte word
	prefix32 := make([]byte, 32)
	copy(prefix32, []byte("\x19Ethereum Signed Message:\n32")) // 26 bytes + 6 zero bytes
	copy(mem[0:32], prefix32)

	// mstore(0x1c, messageHash)  — offset 28
	copy(mem[0x1c:0x1c+32], messageHash.Bytes())

	// keccak256(0x00, 0x3c) — 60 bytes
	solHash := crypto.Keccak256Hash(mem[0:0x3c])
	fmt.Printf("\nSolidity assembly simulation:\n")
	fmt.Printf("mem[0:32] after prefix mstore: %s\n", hex.EncodeToString(mem[0:32]))
	fmt.Printf("mem after messageHash mstore at 0x1c:\n")
	fmt.Printf("  mem[0:60] = %s\n", hex.EncodeToString(mem[0:0x3c]))
	fmt.Printf("  length hashed: %d bytes\n", 0x3c)
	fmt.Println("Solidity ethSignedHash:", solHash.Hex())

	fmt.Println("\n=== Comparison ===")
	fmt.Println("Go hash:       ", goHash.Hex())
	fmt.Println("Solidity hash: ", solHash.Hex())
	fmt.Println("Match:", goHash.Hex() == solHash.Hex())

	if goHash.Hex() != solHash.Hex() {
		fmt.Println("\n*** MISMATCH! The Go EIP-191 prefix implementation differs from Solidity's assembly! ***")
		fmt.Println("Go concats 26 + 32 = 58 bytes")
		fmt.Println("Solidity uses overlapping mstore to produce 60 bytes")
		fmt.Println("The Solidity assembly has the prefix bytes 0-27, then messageHash at bytes 28-59")
		fmt.Println("The prefix '\x19Ethereum Signed Message:\n32' occupies bytes 0-25, bytes 26-27 are zero")
		fmt.Println("Then messageHash overwrites bytes 28-59")
		fmt.Println("So the effective data is: prefix(26 bytes) + 0x0000(2 bytes) + messageHash(32 bytes) = 60 bytes")
	}

	// Method C: Match Solidity exactly — 26 byte prefix + 2 zero bytes + 32 byte hash = 60 bytes
	solCorrect := make([]byte, 0, 60)
	solCorrect = append(solCorrect, []byte("\x19Ethereum Signed Message:\n32")...) // 26
	solCorrect = append(solCorrect, 0, 0)                                          // 2 padding
	solCorrect = append(solCorrect, messageHash.Bytes()...)                        // 32
	solCorrectHash := crypto.Keccak256Hash(solCorrect)
	fmt.Println("\nMethod C (26+2+32=60):", solCorrectHash.Hex())
	fmt.Println("Match Solidity:", solCorrectHash.Hex() == solHash.Hex())
}
