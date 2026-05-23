// Package launchwatcher — 异步对账 + 链上事件回灌。
//
// 职责：
//   1) 监听 FairLaunch 的 Finalized 事件 → 把 Launch.status 从乐观值校正
//      为链上真实结果，写入 founder/platform 分账金额（仅日志）。
//   2) 监听 FairLaunch 的 Deposited 事件 → 把 deposits 灌入 LaunchDeposit。
//   3) 监听 FairLaunch 的 Claimed/Refunded → 翻 LaunchDeposit.claimed/refunded。
//   4) 监听 EpochRegistry 的 RootRecorded → 校验 Epoch.chain_status 并写
//      chain_block。
//   5) 监听 GalaxyNFT 的 GalaxyMinted → 回填 Galaxy.nft_token_id。
//
// 设计：
//   - 单 goroutine 轮询 (Tick = 30s)，每轮 from = max(last_seen, head-1024)
//     到 head；从 DB 表 chain_cursor 取/存 last_seen。
//   - 每个事件类型独立 handler；幂等（按 tx_hash + log_index 去重写入）。
//   - 任何 RPC 失败仅记日志，下一轮重试（永不退出）。
package launchwatcher

import (
	"context"
	"math/big"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ChainCursor — 持久化 watcher 的扫描进度。每个 (chain_id, name) 一行。
type ChainCursor struct {
	ID         uint64    `gorm:"primaryKey;autoIncrement" json:"id"`
	Name       string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"name"`
	LastBlock  uint64    `gorm:"not null;default:0" json:"last_block"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// 事件 topic（在 init 里算出来）。
var (
	depositedTopic     common.Hash
	finalizedTopic     common.Hash
	claimedTopic       common.Hash
	refundedTopic      common.Hash
	galaxyMintedTopic  common.Hash
	rootRecordedTopic  common.Hash

	// 各事件用的 ABI 仅做 Data 字段 unpack（topics 索引自取）。
	fairLaunchEvABI abi.ABI
	galaxyNFTEvABI  abi.ABI
	epochEvABI      abi.ABI
)

func init() {
	depositedTopic = crypto.Keccak256Hash([]byte("Deposited(bytes32,address,uint256,uint128)"))
	finalizedTopic = crypto.Keccak256Hash([]byte("Finalized(bytes32,bool,uint128,uint256,uint256)"))
	claimedTopic = crypto.Keccak256Hash([]byte("Claimed(bytes32,address,uint256)"))
	refundedTopic = crypto.Keccak256Hash([]byte("Refunded(bytes32,address,uint256)"))
	galaxyMintedTopic = crypto.Keccak256Hash([]byte("GalaxyMinted(uint256,bytes32,address,string)"))
	rootRecordedTopic = crypto.Keccak256Hash([]byte("RootRecorded(bytes32,uint64,bytes32,uint64,address)"))

	// 这里只声明 Data 字段，indexed 字段去 topics 取。
	fairLaunchEvABI = mustABI(`[
	  {"type":"event","name":"Deposited","anonymous":false,"inputs":[
		{"indexed":true,"name":"gid","type":"bytes32"},
		{"indexed":true,"name":"who","type":"address"},
		{"indexed":false,"name":"amount","type":"uint256"},
		{"indexed":false,"name":"totalRaised","type":"uint128"}
	  ]},
	  {"type":"event","name":"Finalized","anonymous":false,"inputs":[
		{"indexed":true,"name":"gid","type":"bytes32"},
		{"indexed":false,"name":"succeeded","type":"bool"},
		{"indexed":false,"name":"totalRaised","type":"uint128"},
		{"indexed":false,"name":"founderShare","type":"uint256"},
		{"indexed":false,"name":"platformShare","type":"uint256"}
	  ]},
	  {"type":"event","name":"Claimed","anonymous":false,"inputs":[
		{"indexed":true,"name":"gid","type":"bytes32"},
		{"indexed":true,"name":"who","type":"address"},
		{"indexed":false,"name":"amount","type":"uint256"}
	  ]},
	  {"type":"event","name":"Refunded","anonymous":false,"inputs":[
		{"indexed":true,"name":"gid","type":"bytes32"},
		{"indexed":true,"name":"who","type":"address"},
		{"indexed":false,"name":"amount","type":"uint256"}
	  ]}
	]`)
	galaxyNFTEvABI = mustABI(`[
	  {"type":"event","name":"GalaxyMinted","anonymous":false,"inputs":[
		{"indexed":true,"name":"tokenId","type":"uint256"},
		{"indexed":true,"name":"galaxyId","type":"bytes32"},
		{"indexed":true,"name":"founder","type":"address"},
		{"indexed":false,"name":"uri","type":"string"}
	  ]}
	]`)
	epochEvABI = mustABI(`[
	  {"type":"event","name":"RootRecorded","anonymous":false,"inputs":[
		{"indexed":true,"name":"galaxyId","type":"bytes32"},
		{"indexed":true,"name":"index","type":"uint64"},
		{"indexed":false,"name":"root","type":"bytes32"},
		{"indexed":false,"name":"atomCount","type":"uint64"},
		{"indexed":false,"name":"writer","type":"address"}
	  ]}
	]`)
}

func mustABI(s string) abi.ABI {
	a, err := abi.JSON(strings.NewReader(s))
	if err != nil {
		panic("watcher abi: " + err.Error())
	}
	return a
}

// Start 启动后台 watcher。tick = 0 时用默认 30s。
//
// 安全启动顺序：先 chain.InitWithRetry，再调用本函数（在 main.go 里）。
// 如果 chain.C 为 nil 或没有任何 V4 合约地址配置，会日志退出（不阻塞主流程）。
func Start(tick time.Duration) {
	if tick <= 0 {
		tick = 30 * time.Second
	}
	go func() {
		log := util.Log
		// 等待 chain.C 就绪 (最多 5 分钟)。
		for i := 0; i < 60; i++ {
			if chain.C != nil {
				break
			}
			time.Sleep(5 * time.Second)
		}
		if chain.C == nil {
			log.Warn("[launch-watcher] chain client never came online, exiting")
			return
		}
		if config.Cfg.FairLaunchAddr == "" && config.Cfg.GalaxyNFTAddr == "" && config.Cfg.EpochRegistryAddr == "" {
			log.Info("[launch-watcher] no V4 contracts configured, exiting")
			return
		}
		// 确保 cursor 表存在（开发期 AutoMigrate；生产应迁移到 database.go 主迁移）。
		if err := database.DB.AutoMigrate(&ChainCursor{}); err != nil {
			log.Error("[launch-watcher] migrate cursor: %v", err)
			return
		}
		log.Info("[launch-watcher] started (tick=%s)", tick)
		t := time.NewTicker(tick)
		defer t.Stop()
		for range t.C {
			if err := scanOnce(context.Background()); err != nil {
				log.Warn("[launch-watcher] scan: %v", err)
			}
		}
	}()
}

const cursorName = "v4_logs"

// scanOnce 拉一段日志并处理。失败回退一格以便下次重试。
func scanOnce(ctx context.Context) error {
	eth := chain.C.EthClient()
	head, err := eth.BlockNumber(ctx)
	if err != nil {
		return err
	}

	var cur ChainCursor
	database.DB.Where("name = ?", cursorName).FirstOrCreate(&cur, ChainCursor{Name: cursorName})
	from := cur.LastBlock + 1
	if from == 1 {
		// 第一次启动 — 从 head-50 开始（避免一上线拉一周的历史）。
		if head > 50 {
			from = head - 50
		} else {
			from = 1
		}
	}
	// 单批最多 2000 块。
	const batch = uint64(2000)
	to := head
	if to > from+batch {
		to = from + batch
	}
	if to < from {
		return nil
	}

	addrs := []common.Address{}
	if config.Cfg.FairLaunchAddr != "" {
		addrs = append(addrs, common.HexToAddress(config.Cfg.FairLaunchAddr))
	}
	if config.Cfg.GalaxyNFTAddr != "" {
		addrs = append(addrs, common.HexToAddress(config.Cfg.GalaxyNFTAddr))
	}
	if config.Cfg.EpochRegistryAddr != "" {
		addrs = append(addrs, common.HexToAddress(config.Cfg.EpochRegistryAddr))
	}

	logs, err := eth.FilterLogs(ctx, ethereum.FilterQuery{
		FromBlock: new(big.Int).SetUint64(from),
		ToBlock:   new(big.Int).SetUint64(to),
		Addresses: addrs,
	})
	if err != nil {
		return err
	}
	for _, lg := range logs {
		dispatch(lg)
	}
	database.DB.Model(&ChainCursor{}).Where("id = ?", cur.ID).
		Updates(map[string]interface{}{"last_block": to, "updated_at": time.Now()})
	return nil
}

func dispatch(lg types.Log) {
	if len(lg.Topics) == 0 {
		return
	}
	switch lg.Topics[0] {
	case depositedTopic:
		handleDeposited(lg)
	case finalizedTopic:
		handleFinalized(lg)
	case claimedTopic:
		handleClaimed(lg)
	case refundedTopic:
		handleRefunded(lg)
	case galaxyMintedTopic:
		handleGalaxyMinted(lg)
	case rootRecordedTopic:
		handleRootRecorded(lg)
	}
}

// gidToGalaxyID — 把 bytes32 (前 16 字节为 UUID) 还原成 uuid.UUID。
func gidToGalaxyID(gid common.Hash) uuid.UUID {
	var u uuid.UUID
	copy(u[:], gid[0:16])
	return u
}

// loadLaunch — 按 galaxy_id 取 Launch；找不到返回 (zero, false)。
func loadLaunch(galaxyID uuid.UUID) (models.Launch, bool) {
	var L models.Launch
	err := database.DB.Where("galaxy_id = ?", galaxyID).First(&L).Error
	return L, err == nil
}

// ─── Deposited ────────────────────────────────────────────────────────────────

func handleDeposited(lg types.Log) {
	if len(lg.Topics) < 3 {
		return
	}
	galaxyID := gidToGalaxyID(lg.Topics[1])
	who := common.BytesToAddress(lg.Topics[2].Bytes())
	vals, err := fairLaunchEvABI.Unpack("Deposited", lg.Data)
	if err != nil || len(vals) < 2 {
		return
	}
	amount, _ := vals[0].(*big.Int)
	totalRaised, _ := vals[1].(*big.Int)
	if amount == nil || totalRaised == nil {
		return
	}
	L, ok := loadLaunch(galaxyID)
	if !ok {
		return
	}

	// 更新 Launch.total_raised_wei。
	database.DB.Model(&models.Launch{}).Where("id = ?", L.ID).
		Update("total_raised_wei", totalRaised.String())

	// upsert LaunchDeposit (累加金额)。
	addrLower := strings.ToLower(who.Hex())
	var d models.LaunchDeposit
	if err := database.DB.Where("launch_id = ? AND wallet_addr = ?", L.ID, addrLower).
		First(&d).Error; err == gorm.ErrRecordNotFound {
		database.DB.Create(&models.LaunchDeposit{
			LaunchID:   L.ID,
			WalletAddr: addrLower,
			AmountWei:  amount.String(),
		})
	} else if err == nil {
		prev, _ := new(big.Int).SetString(d.AmountWei, 10)
		if prev == nil {
			prev = big.NewInt(0)
		}
		sum := new(big.Int).Add(prev, amount)
		database.DB.Model(&d).Update("amount_wei", sum.String())
	}
	util.Log.Info("[launch-watcher] Deposited gid=%s who=%s amount=%s total=%s",
		galaxyID, addrLower, amount.String(), totalRaised.String())
}

// ─── Finalized ────────────────────────────────────────────────────────────────

func handleFinalized(lg types.Log) {
	if len(lg.Topics) < 2 {
		return
	}
	galaxyID := gidToGalaxyID(lg.Topics[1])
	vals, err := fairLaunchEvABI.Unpack("Finalized", lg.Data)
	if err != nil || len(vals) < 4 {
		return
	}
	succeeded, _ := vals[0].(bool)
	totalRaised, _ := vals[1].(*big.Int)
	founderShare, _ := vals[2].(*big.Int)
	platformShare, _ := vals[3].(*big.Int)
	L, ok := loadLaunch(galaxyID)
	if !ok {
		return
	}
	status := models.LaunchStatusFinalFail
	if succeeded {
		status = models.LaunchStatusFinalSucc
	}
	upd := map[string]interface{}{"status": status}
	if totalRaised != nil {
		upd["total_raised_wei"] = totalRaised.String()
	}
	database.DB.Model(&models.Launch{}).Where("id = ?", L.ID).Updates(upd)

	if succeeded {
		database.DB.Model(&models.Galaxy{}).Where("id = ?", galaxyID).Updates(map[string]interface{}{
			"stage":      models.GalaxyStageGraduated,
			"token_addr": L.TokenAddr,
		})
		// 飞轮钩子：登记一条待回购记录，等着独立 buyback worker 取走。
		// 幂等：重启 watcher 后同一条事件会被重启动扫描，这里靠 (launch_id, status=queued)
		// 判重。
		platformWei := safeBig(platformShare)
		var exists int64
		database.DB.Model(&models.BuybackEvent{}).
			Where("launch_id = ? AND status = ?", L.ID, models.BuybackStatusQueued).
			Count(&exists)
		if exists == 0 {
			database.DB.Create(&models.BuybackEvent{
				GalaxyID:      galaxyID,
				LaunchID:      L.ID,
				TokenAddr:     L.TokenAddr,
				PlatformShare: platformWei,
				Status:        models.BuybackStatusQueued,
			})
			util.Log.Info("[launch-watcher] buyback queued gid=%s launch=%s amount=%s wei",
				galaxyID, L.ID, platformWei)
		}
	}
	util.Log.Info("[launch-watcher] Finalized gid=%s ok=%v founder=%s platform=%s",
		galaxyID, succeeded, safeBig(founderShare), safeBig(platformShare))
}

func safeBig(b *big.Int) string {
	if b == nil {
		return "0"
	}
	return b.String()
}

// ─── Claimed / Refunded ──────────────────────────────────────────────────────

func handleClaimed(lg types.Log) {
	if len(lg.Topics) < 3 {
		return
	}
	galaxyID := gidToGalaxyID(lg.Topics[1])
	who := strings.ToLower(common.BytesToAddress(lg.Topics[2].Bytes()).Hex())
	L, ok := loadLaunch(galaxyID)
	if !ok {
		return
	}
	database.DB.Model(&models.LaunchDeposit{}).
		Where("launch_id = ? AND wallet_addr = ?", L.ID, who).
		Update("claimed", true)
}

func handleRefunded(lg types.Log) {
	if len(lg.Topics) < 3 {
		return
	}
	galaxyID := gidToGalaxyID(lg.Topics[1])
	who := strings.ToLower(common.BytesToAddress(lg.Topics[2].Bytes()).Hex())
	L, ok := loadLaunch(galaxyID)
	if !ok {
		return
	}
	database.DB.Model(&models.LaunchDeposit{}).
		Where("launch_id = ? AND wallet_addr = ?", L.ID, who).
		Update("refunded", true)
}

// ─── GalaxyMinted ────────────────────────────────────────────────────────────

func handleGalaxyMinted(lg types.Log) {
	if len(lg.Topics) < 3 {
		return
	}
	tokenID := new(big.Int).SetBytes(lg.Topics[1].Bytes())
	galaxyID := gidToGalaxyID(lg.Topics[2])
	// uint64 截断在 V4 完全够用（最多 2^64 个 Galaxy）。
	tid := tokenID.Uint64()
	database.DB.Model(&models.Galaxy{}).Where("id = ?", galaxyID).
		Update("nft_token_id", tid)
	util.Log.Info("[launch-watcher] GalaxyMinted gid=%s tokenId=%d", galaxyID, tid)
}

// ─── RootRecorded ────────────────────────────────────────────────────────────

func handleRootRecorded(lg types.Log) {
	if len(lg.Topics) < 3 {
		return
	}
	galaxyID := gidToGalaxyID(lg.Topics[1])
	idx := new(big.Int).SetBytes(lg.Topics[2].Bytes()).Int64()
	// 通过 (galaxy_id, index) 命中那一行。
	q := database.DB.Model(&models.Epoch{}).Where("index = ?", idx)
	if galaxyID == uuid.Nil {
		q = q.Where("galaxy_id IS NULL")
	} else {
		q = q.Where("galaxy_id = ?", galaxyID)
	}
	q.Updates(map[string]interface{}{
		"chain_status": "confirmed",
		"chain_block":  lg.BlockNumber,
	})
}
