package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/gin-gonic/gin"
)

// AuthWallet verifies that the request includes a valid EIP-191 personal_sign
// signature proving the caller controls the claimed wallet address.
//
// Expected headers:
//   - X-Wallet-Address: the claimed 0x address
//   - X-Wallet-Signature: the hex-encoded signature of the signed message
//
// The signed message must be: "ensoul:mint:<handle>" where <handle> is from the
// JSON body field "handle". This prevents replay of signatures across handles.
func AuthWallet() gin.HandlerFunc {
	return func(c *gin.Context) {
		address := c.GetHeader("X-Wallet-Address")
		signature := c.GetHeader("X-Wallet-Signature")

		if address == "" || signature == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Wallet authentication required (X-Wallet-Address, X-Wallet-Signature)"})
			c.Abort()
			return
		}

		// Validate address format
		if !common.IsHexAddress(address) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid wallet address format"})
			c.Abort()
			return
		}

		// Store the verified address in context for the handler to use
		c.Set("wallet_address", common.HexToAddress(address))
		c.Next()
	}
}

// VerifyWalletSignature recovers the signer address from an EIP-191 personal_sign
// signature and checks it matches the claimed address.
//
// message: the raw message that was signed (e.g., "ensoul:mint:elonmusk")
// sigHex:  the hex-encoded signature (with or without 0x prefix)
// claimed: the address that claims to have signed
func VerifyWalletSignature(message string, sigHex string, claimed common.Address) error {
	// Remove 0x prefix
	sigHex = strings.TrimPrefix(sigHex, "0x")

	// Decode hex signature
	sigBytes := common.FromHex("0x" + sigHex)
	if len(sigBytes) != 65 {
		return fmt.Errorf("invalid signature length: expected 65, got %d", len(sigBytes))
	}

	// EIP-191 personal_sign prefix
	prefixed := fmt.Sprintf("\x19Ethereum Signed Message:\n%d%s", len(message), message)
	hash := crypto.Keccak256Hash([]byte(prefixed))

	// Adjust V value: MetaMask uses 27/28, go-ethereum expects 0/1
	if sigBytes[64] >= 27 {
		sigBytes[64] -= 27
	}

	// Recover public key
	pubKey, err := crypto.SigToPub(hash.Bytes(), sigBytes)
	if err != nil {
		return fmt.Errorf("ecrecover failed: %w", err)
	}

	// Convert recovered pubkey to address
	recoveredAddr := crypto.PubkeyToAddress(*pubKey)

	// Compare addresses (case-insensitive)
	if recoveredAddr != claimed {
		return fmt.Errorf("signature mismatch: recovered %s, claimed %s", recoveredAddr.Hex(), claimed.Hex())
	}

	return nil
}
