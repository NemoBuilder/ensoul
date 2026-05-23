// V4 — Galaxy fair-launch persistence.
//
// One Launch row per Galaxy (1:1, enforced by uniqueIndex on GalaxyID). It
// mirrors the on-chain state machine in contracts/EnsoulFairLaunch.sol:
//
//   draft → open (window active) → finalized:succeeded  → claim-only
//                                  finalized:failed     → refund-only
//
// LaunchDeposit is a denormalised mirror of on-chain deposits, populated by
// an event listener. The on-chain mapping is source of truth; we keep this
// table so the UI can show "your contribution / your projected allocation"
// without hitting RPC for every page load.
package models

import (
	"time"

	"github.com/google/uuid"
)

// ─── Launch status ───────────────────────────────────────────────────────────

const (
	LaunchStatusDraft     = "draft"     // row created, not yet on-chain
	LaunchStatusOpen      = "open"      // openLaunch tx confirmed, depositing
	LaunchStatusFinalSucc = "succeeded" // finalize tx confirmed, claim phase
	LaunchStatusFinalFail = "failed"    // finalize tx confirmed, refund phase
)

// Launch is the off-chain record for one Galaxy's fair-launch.
type Launch struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GalaxyID   uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex" json:"galaxy_id"`
	FounderID  uuid.UUID  `gorm:"type:uuid;not null;index" json:"founder_id"`

	// Window (mirrors on-chain start/end as unix seconds).
	StartAt    time.Time  `gorm:"not null" json:"start_at"`
	EndAt      time.Time  `gorm:"not null" json:"end_at"`

	// Economics — values denominated in wei (BNB has 18 decimals).
	MinRaiseWei string     `gorm:"type:varchar(80);not null" json:"min_raise_wei"`
	MaxRaiseWei string     `gorm:"type:varchar(80);not null;default:'0'" json:"max_raise_wei"` // "0" = uncapped
	TotalRaisedWei string  `gorm:"type:varchar(80);not null;default:'0'" json:"total_raised_wei"`

	// Token supply distributed pro-rata to depositors (uint256 stringified).
	SupplyWei  string     `gorm:"type:varchar(80);not null" json:"supply_wei"`

	// Status mirror.
	Status     string     `gorm:"type:varchar(16);not null;default:'draft';index" json:"status"`

	// On-chain hashes for each transition.
	OpenTxHash      string `gorm:"type:varchar(80);index" json:"open_tx_hash,omitempty"`
	TokenDeployTx   string `gorm:"type:varchar(80);index" json:"token_deploy_tx,omitempty"`
	SetTokenTxHash  string `gorm:"type:varchar(80);index" json:"set_token_tx_hash,omitempty"`
	FinalizeTxHash  string `gorm:"type:varchar(80);index" json:"finalize_tx_hash,omitempty"`

	// Contract addresses (filled as steps complete).
	TokenAddr  string     `gorm:"type:varchar(64);index" json:"token_addr,omitempty"`
	TokenName  string     `gorm:"type:varchar(64)" json:"token_name,omitempty"`
	TokenSymbol string    `gorm:"type:varchar(16)" json:"token_symbol,omitempty"`

	CreatedAt  time.Time  `json:"created_at"`
	UpdatedAt  time.Time  `json:"updated_at"`
}

// LaunchDeposit mirrors on-chain deposits[gid][user] for UI lookups.
// Source of truth remains the chain; this row is refreshed by the event
// indexer (Phase 3.x) or read-on-demand for the connected wallet.
type LaunchDeposit struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	LaunchID   uuid.UUID  `gorm:"type:uuid;not null;index:idx_launch_user,priority:1" json:"launch_id"`
	WalletAddr string     `gorm:"type:varchar(42);not null;index:idx_launch_user,priority:2" json:"wallet_addr"`
	AmountWei  string     `gorm:"type:varchar(80);not null;default:'0'" json:"amount_wei"`
	Claimed    bool       `gorm:"default:false" json:"claimed"`
	Refunded   bool       `gorm:"default:false" json:"refunded"`
	UpdatedAt  time.Time  `json:"updated_at"`
}
