# EnsoulMinter — Wrapper Contract

## Overview

`EnsoulMinter` is a lightweight Solidity wrapper around the ERC-8004 `IdentityRegistry` that adds a BNB mint fee.

### Flow

1. User calls `EnsoulMinter.mint(agentURI)` with `msg.value >= mintFee`
2. Contract calls `IdentityRegistry.register(agentURI)` — NFT is minted to the wrapper contract
3. Contract transfers the NFT to the user via `safeTransferFrom`
4. BNB fee is forwarded to the treasury address

All happens in a single atomic transaction.

## Deployment

### Prerequisites

- [Foundry](https://book.getfoundry.sh/getting-started/installation) or [Hardhat](https://hardhat.org/)
- BNB Smart Chain RPC access
- Deployer wallet with BNB for gas

### Using Foundry (forge)

```bash
# Install dependencies
forge install OpenZeppelin/openzeppelin-contracts

# Compile
forge build

# Deploy (BSC Mainnet)
forge create contracts/EnsoulMinter.sol:EnsoulMinter \
  --rpc-url https://bsc-dataseed.binance.org/ \
  --private-key $DEPLOYER_KEY \
  --constructor-args \
    0x8004A169FB4a3325136EB29fA0ceB6D2e539a432 \  # IdentityRegistry address
    $TREASURY_ADDRESS \                             # Treasury wallet
    1430000000000000                                # 0.00143 BNB ≈ $1 @700U

# Verify on BscScan
forge verify-contract $DEPLOYED_ADDRESS \
  contracts/EnsoulMinter.sol:EnsoulMinter \
  --chain bsc \
  --constructor-args $(cast abi-encode "constructor(address,address,uint256)" \
    0x8004A169FB4a3325136EB29fA0ceB6D2e539a432 \
    $TREASURY_ADDRESS \
    1430000000000000)
```

### Configuration

After deployment, set the environment variable in the web app:

```bash
# .env.local
NEXT_PUBLIC_MINTER_ADDRESS=0x<deployed_address>
```

### Admin Functions

| Function | Description |
|---|---|
| `setMintFee(uint256)` | Update the mint fee (in wei) |
| `setTreasury(address)` | Change the treasury address |
| `setPaused(bool)` | Pause/unpause minting |
| `emergencyWithdraw()` | Recover any stuck BNB |

### Fee Calculation

BNB price fixed at **700 USD/BNB**:

- $1 fee = 1/700 BNB = 0.00143 BNB = `1430000000000000` wei

If need to adjust, call `setMintFee(newFeeInWei)` from the owner wallet.
