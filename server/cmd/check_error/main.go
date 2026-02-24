package main

import (
	"fmt"

	"github.com/ethereum/go-ethereum/crypto"
)

func main() {
	errors := map[string]string{
		// EnsoulMinterV2 custom errors
		"InvalidSignature()":               "MinterV2",
		"ExpiredPermit()":                  "MinterV2",
		"NonceAlreadyUsed()":               "MinterV2",
		"HandleAlreadyMinted()":            "MinterV2",
		"InsufficientFee(uint256,uint256)": "MinterV2",
		"MintingPaused()":                  "MinterV2",
		"TransferFailed()":                 "MinterV2",
		"ZeroAddress()":                    "MinterV2",
		// OpenZeppelin errors
		"OwnableUnauthorizedAccount(address)":  "OZ Ownable",
		"OwnableInvalidOwner(address)":         "OZ Ownable",
		"ReentrancyGuardReentrantCall()":       "OZ ReentrancyGuard",
		"ECDSAInvalidSignature()":              "OZ ECDSA",
		"ECDSAInvalidSignatureLength(uint256)": "OZ ECDSA",
		"ECDSAInvalidSignatureS(bytes32)":      "OZ ECDSA",
		// ERC721 errors
		"ERC721InvalidOwner(address)":                   "ERC721",
		"ERC721NonexistentToken(uint256)":               "ERC721",
		"ERC721IncorrectOwner(address,uint256,address)": "ERC721",
		"ERC721InvalidSender(address)":                  "ERC721",
		"ERC721InvalidReceiver(address)":                "ERC721",
		"ERC721InsufficientApproval(address,uint256)":   "ERC721",
		"ERC721InvalidApprover(address)":                "ERC721",
		"ERC721InvalidOperator(address)":                "ERC721",
		// Generic
		"FailedCall()":                         "OZ Address",
		"InsufficientBalance(uint256,uint256)": "OZ Errors",
		"FailedDeployment()":                   "OZ Errors",
		"MissingPrecompile(address)":           "OZ Errors",
		// Proxy errors
		"ERC1967InvalidImplementation(address)": "Proxy",
		"ERC1967NonPayable()":                   "Proxy",
	}

	target := "cab64075"

	for sig, source := range errors {
		sel := fmt.Sprintf("%x", crypto.Keccak256([]byte(sig))[:4])
		if sel == target {
			fmt.Printf("✅ MATCH: %s -> %s (source: %s)\n", sel, sig, source)
		}
		fmt.Printf("  %s -> %s [%s]\n", sel, sig, source)
	}

	fmt.Printf("\nTarget: 0x%s\n", target)
	fmt.Println("If no match found, it may be a custom error or a require/revert string")

	// Check common Solidity revert patterns
	// 0x08c379a0 = Error(string) - standard revert
	// 0x4e487b71 = Panic(uint256)
	fmt.Println("\nStandard selectors:")
	fmt.Printf("  %x -> Error(string)\n", crypto.Keccak256([]byte("Error(string)"))[:4])
	fmt.Printf("  %x -> Panic(uint256)\n", crypto.Keccak256([]byte("Panic(uint256)"))[:4])
}
