package main

import (
	"context"
	"encoding/hex"
	"fmt"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	rpcURL         = "https://bsc-dataseed.binance.org/"
	minterV2Addr   = "0xc5aE375Dfd8042e9345F1bB8e3b039b6d4690023"
	registryAddr   = "0x8004A169FB4a3325136EB29fA0ceB6D2e539c432"
	platformKeyHex = "c6335b98269a0574746ed6b613fe3d0a99da7c47a0e4cb9be8c120e5d3ae0173"
	callerKeyHex   = "f754c00ba5010057c952e1c673c2ff743872320b16c24c7e88130aa33ea4a74c"
	handle         = "web3leaf" // lowercase
)

func main() {
	ctx := context.Background()

	fmt.Println("========================================")
	fmt.Println("  Ensoul Mint E2E Test")
	fmt.Println("========================================")

	// ── 1. Connect ──────────────────────────────────────────────
	client, err := ethclient.Dial(rpcURL)
	fatal("connect", err)
	defer client.Close()

	chainID, err := client.ChainID(ctx)
	fatal("chainID", err)
	fmt.Println("Chain ID:", chainID)

	// ── 2. Parse keys ──────────────────────────────────────────
	platformKey, err := crypto.HexToECDSA(platformKeyHex)
	fatal("parse platform key", err)
	platformAddr := crypto.PubkeyToAddress(platformKey.PublicKey)
	fmt.Println("Platform (trustedSigner):", platformAddr.Hex())

	callerKey, err := crypto.HexToECDSA(callerKeyHex)
	fatal("parse caller key", err)
	callerAddr := crypto.PubkeyToAddress(callerKey.PublicKey)
	fmt.Println("Caller (user):", callerAddr.Hex())

	// Check caller balance
	bal, err := client.BalanceAt(ctx, callerAddr, nil)
	fatal("balance", err)
	fmt.Printf("Caller BNB balance: %s wei (%.4f BNB)\n", bal.String(), weiToBNB(bal))

	minter := common.HexToAddress(minterV2Addr)

	// ── 3. Read contract state ─────────────────────────────────
	fmt.Println("\n--- Contract State ---")

	trustedSigner := readAddress(ctx, client, minter, "trustedSigner()")
	fmt.Println("trustedSigner:", trustedSigner.Hex())
	if trustedSigner != platformAddr {
		fmt.Println("⚠️  WARNING: trustedSigner != platformAddr!")
	} else {
		fmt.Println("✅ trustedSigner matches platform wallet")
	}

	registry := readAddress(ctx, client, minter, "registry()")
	fmt.Println("registry:", registry.Hex())

	treasury := readAddress(ctx, client, minter, "treasury()")
	fmt.Println("treasury:", treasury.Hex())

	pausedData, _ := client.CallContract(ctx, ethereum.CallMsg{
		To: &minter, Data: crypto.Keccak256([]byte("paused()"))[:4],
	}, nil)
	isPaused := len(pausedData) >= 32 && pausedData[31] == 1
	fmt.Println("paused:", isPaused)
	if isPaused {
		fmt.Println("❌ Contract is PAUSED!")
		os.Exit(1)
	}

	// Check if handle already minted
	handleHash := crypto.Keccak256Hash([]byte(handle))
	fmt.Println("\nHandle:", handle)
	fmt.Println("HandleHash:", handleHash.Hex())

	// ── 4. Build permit ────────────────────────────────────────────────
	fmt.Println("\n--- Building Permit ---")

	priceWei := big.NewInt(10000000000000000) // 0.01 BNB (tier: < 1K followers)
	deadline := time.Now().Unix() + 1800      // 30 minutes from now
	nonce := uint64(time.Now().UnixNano() % 1000000000)

	fmt.Printf("Price: %s wei (%.4f BNB)\n", priceWei.String(), weiToBNB(priceWei))
	fmt.Println("Deadline:", deadline, "(", time.Unix(deadline, 0).Format(time.RFC3339), ")")
	fmt.Println("Nonce:", nonce)

	// abi.encodePacked(handleHash, price, msg.sender, deadline, nonce, chainid, minterAddr)
	packed := []byte{}
	packed = append(packed, handleHash.Bytes()...)                                             // bytes32: 32
	packed = append(packed, common.LeftPadBytes(priceWei.Bytes(), 32)...)                      // uint256: 32
	packed = append(packed, callerAddr.Bytes()...)                                             // address: 20
	packed = append(packed, common.LeftPadBytes(big.NewInt(deadline).Bytes(), 32)...)          // uint256: 32
	packed = append(packed, common.LeftPadBytes(new(big.Int).SetUint64(nonce).Bytes(), 32)...) // uint256: 32
	packed = append(packed, common.LeftPadBytes(chainID.Bytes(), 32)...)                       // uint256: 32
	packed = append(packed, minter.Bytes()...)                                                 // address: 20

	fmt.Printf("Packed length: %d bytes (expect 200)\n", len(packed))
	if len(packed) != 200 {
		fmt.Println("❌ Wrong packed length!")
		os.Exit(1)
	}

	messageHash := crypto.Keccak256Hash(packed)
	fmt.Println("MessageHash:", messageHash.Hex())

	// EIP-191 prefix: "\x19Ethereum Signed Message:\n32"
	prefix := []byte(fmt.Sprintf("\x19Ethereum Signed Message:\n32"))
	fmt.Printf("Prefix length: %d (expect 28)\n", len(prefix))

	prefixed := crypto.Keccak256Hash(append(prefix, messageHash.Bytes()...))
	fmt.Println("EthSignedHash:", prefixed.Hex())

	sig, err := crypto.Sign(prefixed.Bytes(), platformKey)
	fatal("sign", err)
	if sig[64] < 27 {
		sig[64] += 27
	}
	fmt.Println("Signature:", "0x"+hex.EncodeToString(sig))
	fmt.Println("Sig length:", len(sig), "(expect 65)")

	// Verify locally: recover signer
	recoveredPub, err := crypto.Ecrecover(prefixed.Bytes(), sig)
	if err == nil {
		pubKey, _ := crypto.UnmarshalPubkey(recoveredPub)
		recovered := crypto.PubkeyToAddress(*pubKey)
		fmt.Println("Recovered signer:", recovered.Hex())
		if recovered == platformAddr {
			fmt.Println("✅ Signature verification passed locally")
		} else {
			fmt.Println("❌ Signature mismatch! recovered:", recovered.Hex(), "expected:", platformAddr.Hex())
			os.Exit(1)
		}
	}

	// ── 5. Build mint() call ───────────────────────────────────
	fmt.Println("\n--- Building mint() call ---")

	agentURI := "ensoul://soul/" + handle
	fmt.Println("AgentURI:", agentURI)

	mintABI, _ := abi.JSON(strings.NewReader(`[{"type":"function","name":"mint","inputs":[{"name":"agentURI","type":"string"},{"name":"handleHash","type":"bytes32"},{"name":"price","type":"uint256"},{"name":"deadline","type":"uint256"},{"name":"nonce","type":"uint256"},{"name":"signature","type":"bytes"}],"outputs":[{"name":"agentId","type":"uint256"}],"stateMutability":"payable"}]`))

	mintData, err := mintABI.Pack("mint",
		agentURI,
		handleHash,
		priceWei,
		big.NewInt(deadline),
		new(big.Int).SetUint64(nonce),
		sig,
	)
	fatal("pack mint", err)
	fmt.Println("Calldata length:", len(mintData))

	// ── 6. eth_call simulation ─────────────────────────────────
	fmt.Println("\n--- Simulating mint() via eth_call ---")

	result, err := client.CallContract(ctx, ethereum.CallMsg{
		From:  callerAddr,
		To:    &minter,
		Value: priceWei,
		Data:  mintData,
	}, nil)
	if err != nil {
		fmt.Println("❌ mint() REVERTED:", err)
		// Try to decode the error
		errStr := err.Error()
		if strings.Contains(errStr, "cab64075") {
			fmt.Println("   Error: ExpiredPermit() — deadline has passed")
		} else if strings.Contains(errStr, "8baa579f") {
			fmt.Println("   Error: InvalidSignature() — signature doesn't match trustedSigner")
		} else if strings.Contains(errStr, "1fb09b80") {
			fmt.Println("   Error: NonceAlreadyUsed()")
		} else if strings.Contains(errStr, "eb560756") {
			fmt.Println("   Error: MintingPaused()")
		} else if strings.Contains(errStr, "a458261b") {
			fmt.Println("   Error: InsufficientFee()")
		} else if strings.Contains(errStr, "90b8ec18") {
			fmt.Println("   Error: TransferFailed()")
		} else if strings.Contains(errStr, "64a0ae92") {
			fmt.Println("   Error: ERC721InvalidReceiver()")
		}
		fmt.Println("\n   Aborting — NOT sending real transaction.")
		os.Exit(1)
	}

	agentId := new(big.Int).SetBytes(result)
	fmt.Println("✅ eth_call succeeded! Predicted agentId:", agentId)

	// ── 7. Send real transaction ───────────────────────────────
	fmt.Println("\n--- Sending REAL transaction ---")
	fmt.Println("⚠️  This will spend real BNB!")

	txNonce, err := client.PendingNonceAt(ctx, callerAddr)
	fatal("nonce", err)

	gasPrice, err := client.SuggestGasPrice(ctx)
	fatal("gasPrice", err)
	fmt.Println("Gas price:", gasPrice, "wei")

	// Estimate gas
	gasLimit, err := client.EstimateGas(ctx, ethereum.CallMsg{
		From:  callerAddr,
		To:    &minter,
		Value: priceWei,
		Data:  mintData,
	})
	if err != nil {
		fmt.Println("⚠️  Gas estimation failed:", err, "— using 500000")
		gasLimit = 500000
	} else {
		gasLimit = gasLimit * 120 / 100 // +20% buffer
		fmt.Println("Estimated gas:", gasLimit)
	}

	tx := types.NewTransaction(txNonce, minter, priceWei, gasLimit, gasPrice, mintData)
	signedTx, err := types.SignTx(tx, types.NewEIP155Signer(chainID), callerKey)
	fatal("sign tx", err)

	err = client.SendTransaction(ctx, signedTx)
	fatal("send tx", err)

	txHash := signedTx.Hash().Hex()
	fmt.Println("✅ Transaction sent!")
	fmt.Println("   TX Hash:", txHash)
	fmt.Println("   BSCScan: https://bscscan.com/tx/" + txHash)

	// ── 8. Wait for receipt ────────────────────────────────────
	fmt.Println("\n--- Waiting for confirmation ---")
	for i := 0; i < 60; i++ {
		receipt, err := client.TransactionReceipt(ctx, signedTx.Hash())
		if err == nil {
			if receipt.Status == 1 {
				fmt.Println("✅ Transaction SUCCEEDED!")
				fmt.Println("   Block:", receipt.BlockNumber)
				fmt.Println("   Gas used:", receipt.GasUsed)
				// Parse Registered event
				for _, log := range receipt.Logs {
					if log.Address == registry {
						if len(log.Topics) >= 3 && log.Topics[0].Hex() == crypto.Keccak256Hash([]byte("Registered(uint256,string,address)")).Hex() {
							aid := new(big.Int).SetBytes(log.Topics[1].Bytes())
							fmt.Println("   Agent ID:", aid)
						}
					}
				}
			} else {
				fmt.Println("❌ Transaction FAILED (reverted on-chain)")
				fmt.Println("   Block:", receipt.BlockNumber)
				fmt.Println("   Gas used:", receipt.GasUsed)
			}
			os.Exit(0)
		}
		time.Sleep(3 * time.Second)
		fmt.Printf("   Waiting... (%d/60)\n", i+1)
	}
	fmt.Println("⏰ Timed out waiting for receipt. Check BSCScan manually:")
	fmt.Println("   https://bscscan.com/tx/" + txHash)
}

func readAddress(ctx context.Context, client *ethclient.Client, contract common.Address, funcSig string) common.Address {
	data := crypto.Keccak256([]byte(funcSig))[:4]
	result, err := client.CallContract(ctx, ethereum.CallMsg{To: &contract, Data: data}, nil)
	if err != nil || len(result) < 32 {
		return common.Address{}
	}
	return common.BytesToAddress(result)
}

func weiToBNB(wei *big.Int) float64 {
	f := new(big.Float).SetInt(wei)
	e18 := new(big.Float).SetFloat64(1e18)
	f.Quo(f, e18)
	result, _ := f.Float64()
	return result
}

func fatal(step string, err error) {
	if err != nil {
		fmt.Printf("❌ FATAL [%s]: %v\n", step, err)
		os.Exit(1)
	}
}
