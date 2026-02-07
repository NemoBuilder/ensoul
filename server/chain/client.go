package chain

import (
	"context"
	"crypto/ecdsa"
	"fmt"
	"log"
	"math/big"

	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/contracts"
)

// Client wraps the Ethereum client and contract instances for ERC-8004 interaction.
type Client struct {
	ethClient          *ethclient.Client
	identityRegistry   *contracts.IdentityRegistry
	reputationRegistry *contracts.ReputationRegistry
	platformKey        *ecdsa.PrivateKey
	platformAddr       common.Address
	chainID            *big.Int
}

// Global chain client instance
var C *Client

// Init initializes the blockchain client and contract bindings.
// It connects to the BSC RPC, parses the platform private key, and binds to
// the pre-deployed ERC-8004 IdentityRegistry and ReputationRegistry contracts.
func Init() error {
	cfg := config.Cfg

	// Connect to BSC RPC
	client, err := ethclient.Dial(cfg.BSCRPCURL)
	if err != nil {
		return fmt.Errorf("failed to connect to BSC RPC (%s): %w", cfg.BSCRPCURL, err)
	}

	// Get chain ID for transaction signing
	chainID, err := client.ChainID(context.Background())
	if err != nil {
		return fmt.Errorf("failed to get chain ID: %w", err)
	}
	log.Printf("[chain] Connected to chain ID: %s (RPC: %s)", chainID.String(), cfg.BSCRPCURL)

	// Parse the platform private key (used for minting souls)
	var platformKey *ecdsa.PrivateKey
	var platformAddr common.Address
	if cfg.PrivateKey != "" {
		// Strip "0x" prefix if present
		pkHex := cfg.PrivateKey
		if len(pkHex) > 2 && pkHex[:2] == "0x" {
			pkHex = pkHex[2:]
		}
		platformKey, err = crypto.HexToECDSA(pkHex)
		if err != nil {
			return fmt.Errorf("failed to parse platform private key: %w", err)
		}
		platformAddr = crypto.PubkeyToAddress(platformKey.PublicKey)
		log.Printf("[chain] Platform wallet: %s", platformAddr.Hex())
	} else {
		log.Println("[chain] WARNING: No PLATFORM_PRIVATE_KEY set, on-chain writes will be disabled")
	}

	// Bind to Identity Registry contract
	identityAddr := common.HexToAddress(cfg.IdentityRegistryAddr)
	identityRegistry, err := contracts.NewIdentityRegistry(identityAddr, client)
	if err != nil {
		return fmt.Errorf("failed to bind Identity Registry at %s: %w", identityAddr.Hex(), err)
	}
	log.Printf("[chain] Identity Registry bound: %s", identityAddr.Hex())

	// Bind to Reputation Registry contract
	reputationAddr := common.HexToAddress(cfg.ReputationRegistryAddr)
	reputationRegistry, err := contracts.NewReputationRegistry(reputationAddr, client)
	if err != nil {
		return fmt.Errorf("failed to bind Reputation Registry at %s: %w", reputationAddr.Hex(), err)
	}
	log.Printf("[chain] Reputation Registry bound: %s", reputationAddr.Hex())

	// Verify contracts are accessible by reading version
	version, err := identityRegistry.GetVersion(&bind.CallOpts{})
	if err != nil {
		log.Printf("[chain] WARNING: Could not read Identity Registry version (contract may not be deployed on this chain): %v", err)
	} else {
		log.Printf("[chain] Identity Registry version: %s", version)
	}

	repVersion, err := reputationRegistry.GetVersion(&bind.CallOpts{})
	if err != nil {
		log.Printf("[chain] WARNING: Could not read Reputation Registry version: %v", err)
	} else {
		log.Printf("[chain] Reputation Registry version: %s", repVersion)
	}

	C = &Client{
		ethClient:          client,
		identityRegistry:   identityRegistry,
		reputationRegistry: reputationRegistry,
		platformKey:        platformKey,
		platformAddr:       platformAddr,
		chainID:            chainID,
	}

	return nil
}

// EthClient returns the underlying ethclient for direct use.
func (c *Client) EthClient() *ethclient.Client {
	return c.ethClient
}

// IdentityRegistry returns the Identity Registry contract binding.
func (c *Client) IdentityRegistry() *contracts.IdentityRegistry {
	return c.identityRegistry
}

// ReputationRegistry returns the Reputation Registry contract binding.
func (c *Client) ReputationRegistry() *contracts.ReputationRegistry {
	return c.reputationRegistry
}

// ChainID returns the connected chain's ID.
func (c *Client) ChainID() *big.Int {
	return c.chainID
}

// PlatformAddress returns the platform wallet address.
func (c *Client) PlatformAddress() common.Address {
	return c.platformAddr
}

// HasPlatformKey returns true if a platform private key is configured.
func (c *Client) HasPlatformKey() bool {
	return c.platformKey != nil
}

// PlatformKey returns the platform private key for direct use (e.g., in tests).
func (c *Client) PlatformKey() *ecdsa.PrivateKey {
	return c.platformKey
}

// PlatformTransactOpts creates transaction options signed with the platform key.
// Used for soul minting and metadata updates.
func (c *Client) PlatformTransactOpts(ctx context.Context) (*bind.TransactOpts, error) {
	if c.platformKey == nil {
		return nil, fmt.Errorf("platform private key not configured")
	}
	opts, err := bind.NewKeyedTransactorWithChainID(c.platformKey, c.chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}
	opts.Context = ctx
	return opts, nil
}

// TransactOptsFromKey creates transaction options from a given private key.
// Used for Claw wallet transactions (reputation feedback).
func (c *Client) TransactOptsFromKey(ctx context.Context, key *ecdsa.PrivateKey) (*bind.TransactOpts, error) {
	opts, err := bind.NewKeyedTransactorWithChainID(key, c.chainID)
	if err != nil {
		return nil, fmt.Errorf("failed to create transactor: %w", err)
	}
	opts.Context = ctx
	return opts, nil
}
