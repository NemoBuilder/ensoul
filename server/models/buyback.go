package models

import (
	"time"

	"github.com/google/uuid"
)

// BuybackEvent — Phase 3 飞轮回购队列。
// FairLaunch.Finalized 成功后，watcher 在这里登记一条 platform-share
// 待回购记录；链下 buyback worker 后续读取并执行 PancakeSwap swap，
// 完成后更新 ExecutedTxHash 与 ExecutedAt。
//
// 注意：这是「待办登记表」，不是分润账本。真正的分润逻辑（持有者
// claim / 销毁等）走独立服务，引用本表为信号。
type BuybackEvent struct {
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GalaxyID       uuid.UUID `gorm:"type:uuid;not null;index" json:"galaxy_id"`
	LaunchID       uuid.UUID `gorm:"type:uuid;not null;index" json:"launch_id"`
	TokenAddr      string    `gorm:"type:varchar(64);index" json:"token_addr"`
	PlatformShare  string    `gorm:"type:varchar(80);not null;default:'0'" json:"platform_share_wei"`
	Status         string    `gorm:"type:varchar(16);not null;default:'queued';index" json:"status"` // queued | executed | failed | skipped
	ExecutedTxHash string    `gorm:"type:varchar(80);index" json:"executed_tx_hash,omitempty"`
	ExecutedAt     *time.Time `json:"executed_at,omitempty"`
	Note           string    `gorm:"type:text" json:"note,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

const (
	BuybackStatusQueued   = "queued"
	BuybackStatusExecuted = "executed"
	BuybackStatusFailed   = "failed"
	BuybackStatusSkipped  = "skipped"
)
