// Package launch — fair-launch orchestration.
//
// This wraps three platform-key on-chain ops (openLaunch / setToken /
// finalize) with their off-chain DB bookkeeping and a LaunchReady gate
// that the API layer must call before opening a launch.
//
// The depositor flows (deposit / claim / refund) are signed directly by
// user wallets from the frontend; they never touch this package.
package launch

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services/quality"
	"github.com/ensoul-labs/ensoul-server/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/google/uuid"
)

// Errors surfaced to the handler layer.
var (
	ErrNotLaunchReady   = errors.New("launch: galaxy not LaunchReady")
	ErrAlreadyExists    = errors.New("launch: galaxy already has a launch")
	ErrNotFound         = errors.New("launch: not found")
	ErrBadWindow        = errors.New("launch: window must be in the future and end > start")
	ErrBadAmounts       = errors.New("launch: amounts must be positive (and maxRaise>=minRaise when set)")
	ErrFounderNoWallet  = errors.New("launch: founder has no wallet address")
	ErrTokenAlreadySet  = errors.New("launch: token already wired")
	ErrWrongStatus      = errors.New("launch: wrong status for this transition")
)

// OpenParams are the inputs from the admin handler.
type OpenParams struct {
	GalaxyID     uuid.UUID
	StartAt      time.Time
	EndAt        time.Time
	MinRaiseWei  *big.Int // wei; required
	MaxRaiseWei  *big.Int // wei; nil or zero = uncapped
	SupplyWei    *big.Int // token base units; required
	TokenName    string   // recorded but token deploy is off-band
	TokenSymbol  string
}

// Open creates a Launch row + submits openLaunch() on-chain. Caller must
// already have verified the user is admin. Galaxy must pass LaunchReady.
func Open(ctx context.Context, p OpenParams) (*models.Launch, error) {
	if p.StartAt.After(p.EndAt) || time.Until(p.EndAt) <= 0 {
		return nil, ErrBadWindow
	}
	if p.MinRaiseWei == nil || p.MinRaiseWei.Sign() <= 0 ||
		p.SupplyWei == nil || p.SupplyWei.Sign() <= 0 {
		return nil, ErrBadAmounts
	}
	if p.MaxRaiseWei != nil && p.MaxRaiseWei.Sign() > 0 && p.MaxRaiseWei.Cmp(p.MinRaiseWei) < 0 {
		return nil, ErrBadAmounts
	}

	// Galaxy + founder lookups.
	var galaxy models.Galaxy
	if err := database.DB.First(&galaxy, "id = ?", p.GalaxyID).Error; err != nil {
		return nil, ErrNotFound
	}
	// Idempotency: one launch per galaxy.
	var existing int64
	database.DB.Model(&models.Launch{}).Where("galaxy_id = ?", p.GalaxyID).Count(&existing)
	if existing > 0 {
		return nil, ErrAlreadyExists
	}

	// LaunchReady gate.
	snap, err := quality.Recompute(p.GalaxyID)
	if err != nil {
		return nil, fmt.Errorf("recompute quality: %w", err)
	}
	if !quality.LaunchReady(snap) {
		return nil, ErrNotLaunchReady
	}

	var founder models.User
	if err := database.DB.Select("id, wallet_addr").First(&founder, "id = ?", galaxy.FounderID).Error; err != nil {
		return nil, fmt.Errorf("founder lookup: %w", err)
	}
	if founder.WalletAddr == "" {
		return nil, ErrFounderNoWallet
	}

	// Insert draft row first so a chain failure leaves an auditable trail.
	maxStr := "0"
	if p.MaxRaiseWei != nil {
		maxStr = p.MaxRaiseWei.String()
	}
	row := models.Launch{
		GalaxyID:    p.GalaxyID,
		FounderID:   galaxy.FounderID,
		StartAt:     p.StartAt,
		EndAt:       p.EndAt,
		MinRaiseWei: p.MinRaiseWei.String(),
		MaxRaiseWei: maxStr,
		SupplyWei:   p.SupplyWei.String(),
		Status:      models.LaunchStatusDraft,
		TokenName:   p.TokenName,
		TokenSymbol: p.TokenSymbol,
	}
	if err := database.DB.Create(&row).Error; err != nil {
		return nil, err
	}

	// On-chain open. galaxyId = UUID left-aligned into bytes32.
	var gid [32]byte
	b, _ := p.GalaxyID.MarshalBinary()
	copy(gid[0:16], b)
	maxRaise := big.NewInt(0)
	if p.MaxRaiseWei != nil {
		maxRaise = p.MaxRaiseWei
	}
	tx, err := chain.OpenLaunch(ctx, gid,
		common.HexToAddress(founder.WalletAddr),
		uint64(p.StartAt.Unix()), uint64(p.EndAt.Unix()),
		p.MinRaiseWei, maxRaise, p.SupplyWei)
	if err != nil {
		if errors.Is(err, chain.ErrFairLaunchNotConfigured) {
			util.Log.Warn("[launch] FAIR_LAUNCH_ADDR unset — draft row %s left without chain push", row.ID)
			return &row, nil
		}
		return &row, fmt.Errorf("openLaunch tx: %w", err)
	}

	// Flip row to open + update galaxy stage in one transaction.
	if err := database.DB.Model(&row).Updates(map[string]interface{}{
		"open_tx_hash": tx,
		"status":       models.LaunchStatusOpen,
	}).Error; err != nil {
		return &row, err
	}
	database.DB.Model(&models.Galaxy{}).Where("id = ?", p.GalaxyID).
		Update("stage", models.GalaxyStageRaising)
	row.OpenTxHash = tx
	row.Status = models.LaunchStatusOpen
	return &row, nil
}

// WireToken records a deployed EnsoulCommunityToken address against an
// already-open launch and submits setToken() on-chain. Deployment of the
// token bytecode itself is intentionally out-of-band (forge/Remix); this
// keeps the Go service free of compiled bytecode blobs.
func WireToken(ctx context.Context, galaxyID uuid.UUID, tokenAddr string) (*models.Launch, error) {
	var L models.Launch
	if err := database.DB.Where("galaxy_id = ?", galaxyID).First(&L).Error; err != nil {
		return nil, ErrNotFound
	}
	if L.Status != models.LaunchStatusOpen {
		return nil, ErrWrongStatus
	}
	if L.TokenAddr != "" {
		return nil, ErrTokenAlreadySet
	}
	tokenAddr = strings.TrimSpace(tokenAddr)
	if !common.IsHexAddress(tokenAddr) {
		return nil, fmt.Errorf("bad token address")
	}
	var gid [32]byte
	b, _ := galaxyID.MarshalBinary()
	copy(gid[0:16], b)
	tx, err := chain.SetToken(ctx, gid, common.HexToAddress(tokenAddr))
	if err != nil {
		return &L, fmt.Errorf("setToken tx: %w", err)
	}
	if err := database.DB.Model(&L).Updates(map[string]interface{}{
		"token_addr":         strings.ToLower(tokenAddr),
		"set_token_tx_hash":  tx,
	}).Error; err != nil {
		return &L, err
	}
	L.TokenAddr = strings.ToLower(tokenAddr)
	L.SetTokenTxHash = tx
	return &L, nil
}

// Finalize submits finalize() on-chain. The success/failure outcome only
// becomes authoritative after the receipt is parsed by a watcher; this call
// optimistically marks the row as succeeded if the window closed and a token
// is wired, else failed. The watcher (Phase 3.x) can downgrade.
func Finalize(ctx context.Context, galaxyID uuid.UUID) (*models.Launch, error) {
	var L models.Launch
	if err := database.DB.Where("galaxy_id = ?", galaxyID).First(&L).Error; err != nil {
		return nil, ErrNotFound
	}
	if L.Status != models.LaunchStatusOpen {
		return nil, ErrWrongStatus
	}
	if time.Now().Before(L.EndAt) {
		return nil, fmt.Errorf("launch: window not yet closed")
	}
	var gid [32]byte
	b, _ := galaxyID.MarshalBinary()
	copy(gid[0:16], b)
	tx, err := chain.Finalize(ctx, gid)
	if err != nil {
		return &L, fmt.Errorf("finalize tx: %w", err)
	}

	// Optimistic guess pending receipt parse.
	guess := models.LaunchStatusFinalFail
	if L.TokenAddr != "" {
		guess = models.LaunchStatusFinalSucc
	}
	if err := database.DB.Model(&L).Updates(map[string]interface{}{
		"finalize_tx_hash": tx,
		"status":           guess,
	}).Error; err != nil {
		return &L, err
	}
	if guess == models.LaunchStatusFinalSucc {
		database.DB.Model(&models.Galaxy{}).Where("id = ?", galaxyID).Updates(map[string]interface{}{
			"stage":      models.GalaxyStageGraduated,
			"token_addr": L.TokenAddr,
		})
	}
	L.FinalizeTxHash = tx
	L.Status = guess
	return &L, nil
}
