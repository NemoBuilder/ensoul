package chain

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"io"

	"github.com/ethereum/go-ethereum/crypto"

	"github.com/ensoul-labs/ensoul-server/config"
)

// ClawWallet holds the generated wallet data for a Claw agent.
type ClawWallet struct {
	Address       string // Hex address (0x...)
	PrivateKeyEnc string // AES-GCM encrypted private key (base64)
}

// GenerateClawWallet creates a new Ethereum keypair for a Claw agent.
// The private key is AES-GCM encrypted using CLAW_PK_SECRET before storage.
func GenerateClawWallet() (*ClawWallet, error) {
	// Generate a new random private key
	privateKey, err := crypto.GenerateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Derive public address
	address := crypto.PubkeyToAddress(privateKey.PublicKey)

	// Serialize private key to bytes
	pkBytes := crypto.FromECDSA(privateKey)

	// Encrypt the private key
	encrypted, err := encryptPrivateKey(pkBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to encrypt private key: %w", err)
	}

	return &ClawWallet{
		Address:       address.Hex(),
		PrivateKeyEnc: encrypted,
	}, nil
}

// DecryptClawPrivateKey decrypts a Claw's encrypted private key to use for signing transactions.
func DecryptClawPrivateKey(encryptedPK string) (*ecdsa.PrivateKey, error) {
	pkBytes, err := decryptPrivateKey(encryptedPK)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt private key: %w", err)
	}

	privateKey, err := crypto.ToECDSA(pkBytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return privateKey, nil
}

// GetClawAddress derives the address from an encrypted private key without exposing the key.
func GetClawAddress(encryptedPK string) (string, error) {
	key, err := DecryptClawPrivateKey(encryptedPK)
	if err != nil {
		return "", err
	}
	return crypto.PubkeyToAddress(key.PublicKey).Hex(), nil
}

// encryptPrivateKey encrypts raw private key bytes using AES-256-GCM.
// Returns base64-encoded ciphertext (nonce prepended).
func encryptPrivateKey(plaintext []byte) (string, error) {
	secret := config.Cfg.ClawPKSecret
	if secret == "" {
		// If no secret configured, use a dummy encryption (for dev only)
		return "dev:" + hex.EncodeToString(plaintext), nil
	}

	// Derive 32-byte key from secret (pad/truncate to 32 bytes)
	key := deriveKey(secret)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// decryptPrivateKey decrypts a base64-encoded AES-256-GCM ciphertext.
func decryptPrivateKey(encrypted string) ([]byte, error) {
	secret := config.Cfg.ClawPKSecret
	if secret == "" {
		// Dev mode: handle "dev:" prefix
		if len(encrypted) > 4 && encrypted[:4] == "dev:" {
			return hex.DecodeString(encrypted[4:])
		}
		return nil, fmt.Errorf("CLAW_PK_SECRET not configured")
	}

	ciphertext, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return nil, fmt.Errorf("invalid base64: %w", err)
	}

	key := deriveKey(secret)

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// deriveKey pads or truncates a string secret to exactly 32 bytes for AES-256.
func deriveKey(secret string) []byte {
	key := make([]byte, 32)
	copy(key, []byte(secret))
	return key
}
