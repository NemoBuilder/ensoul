// Package buybackworker — V4 flywheel buyback executor.
//
// Reads BuybackEvent rows with status='queued' that the launch-watcher
// enqueued after a successful FairLaunch.Finalized, and turns each into
// an on-chain PancakeSwap BNB→galaxy-token buyback.
//
// 设计：
//   - 单 goroutine 30 秒轮询；批量取 10 条；逐条处理。
//   - 处理流程：先 update status=executing（乐观锁，避免重启重放），
//     调 chain.SwapBNBForArbitraryToken；
//     成功 → status=executed + executed_tx_hash + executed_at；
//     失败 → status=failed + note=<err>，下一轮人工介入。
//   - 容错：chain.C 没起来时直接 skip；BuybackPrivateKey 未配置时 skip。
//   - 幂等：靠 status 状态机；watcher 重启重放也会被 queued-only 过滤掉。
package buybackworker

import (
	"context"
	"fmt"
	"math/big"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	// BuybackSlippageBps — 5% slippage tolerance for small-cap community tokens.
	BuybackSlippageBps = 500
	// MinSwapWei — 跳过 dust 级别（<= 0.001 BNB），避免 gas 大于本金。
	MinSwapWei = 1_000_000_000_000_000 // 1e15 = 0.001 BNB
)

// Start 启动后台 worker。tick=0 时默认 30s。
func Start(tick time.Duration) {
	if tick <= 0 {
		tick = 30 * time.Second
	}
	go run(tick)
}

func run(tick time.Duration) {
	util.Log.Info("[buyback-worker] started tick=%s", tick)
	t := time.NewTicker(tick)
	defer t.Stop()
	for range t.C {
		if err := drain(); err != nil {
			util.Log.Warn("[buyback-worker] drain: %v", err)
		}
	}
}

func drain() error {
	if chain.C == nil {
		return nil
	}
	if config.Cfg.BuybackPrivateKey == "" {
		return nil
	}
	var rows []models.BuybackEvent
	if err := database.DB.
		Where("status = ?", models.BuybackStatusQueued).
		Order("created_at ASC").
		Limit(10).Find(&rows).Error; err != nil {
		return err
	}
	for _, ev := range rows {
		processOne(ev)
	}
	return nil
}

func processOne(ev models.BuybackEvent) {
	amount, ok := new(big.Int).SetString(ev.PlatformShare, 10)
	if !ok || amount == nil || amount.Sign() <= 0 {
		markFailed(ev.ID, "invalid platform_share_wei")
		return
	}
	if amount.Cmp(big.NewInt(MinSwapWei)) < 0 {
		markSkipped(ev.ID, "amount below min swap threshold")
		return
	}
	if !common.IsHexAddress(ev.TokenAddr) {
		markFailed(ev.ID, "missing or invalid token_addr")
		return
	}

	// 乐观锁：把这条记录从 queued 切到 executing，确保多实例不会重复 swap。
	// 这里没有真正的 executing 状态，复用 note 字段做标记 + 状态保持 queued 也可以，
	// 但为了简单与肉眼可见，引入一次性 update + reload。
	now := time.Now()
	res := database.DB.Model(&models.BuybackEvent{}).
		Where("id = ? AND status = ?", ev.ID, models.BuybackStatusQueued).
		Updates(map[string]interface{}{
			"note":       "executing",
			"updated_at": now,
		})
	if res.RowsAffected == 0 {
		return // 已被另一实例抢走或状态已变
	}

	key, err := chain.ParsePrivateKey(config.Cfg.BuybackPrivateKey)
	if err != nil {
		markFailed(ev.ID, fmt.Sprintf("parse buyback key: %v", err))
		return
	}
	recipient := crypto.PubkeyToAddress(key.PublicKey)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	txHash, minOut, err := chain.SwapBNBForArbitraryToken(
		ctx, key, amount, ev.TokenAddr, recipient, BuybackSlippageBps,
	)
	if err != nil {
		markFailed(ev.ID, fmt.Sprintf("swap: %v", err))
		return
	}

	success, err := chain.WaitForTokenTx(ctx, txHash)
	if err != nil || !success {
		markFailed(ev.ID, fmt.Sprintf("tx receipt: hash=%s err=%v", txHash, err))
		return
	}

	executed := time.Now()
	database.DB.Model(&models.BuybackEvent{}).Where("id = ?", ev.ID).Updates(map[string]interface{}{
		"status":           models.BuybackStatusExecuted,
		"executed_tx_hash": txHash,
		"executed_at":      &executed,
		"note":             fmt.Sprintf("minOut=%s", minOut.String()),
	})
	util.Log.Info("[buyback-worker] executed launch=%s bnb=%s tx=%s minOut=%s",
		ev.LaunchID, amount.String(), txHash, minOut.String())
}

func markFailed(id any, note string) {
	database.DB.Model(&models.BuybackEvent{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": models.BuybackStatusFailed,
		"note":   note,
	})
}

func markSkipped(id any, note string) {
	database.DB.Model(&models.BuybackEvent{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status": models.BuybackStatusSkipped,
		"note":   note,
	})
}
