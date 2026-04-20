package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// VibeWorkspace represents a Vibe Write workspace.
// Free users get 1 workspace, Pro users get up to 10.
type VibeWorkspace struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Name          string         `gorm:"type:varchar(100);not null" json:"name"`
	TwitterHandle string         `gorm:"type:varchar(30)" json:"twitter_handle,omitempty"`
	SortOrder     int            `gorm:"default:0" json:"sort_order"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// Memory category constants
const (
	MemoryCategoryProfile   = "profile"
	MemoryCategoryKnowledge = "knowledge"
	MemoryCategoryNetwork   = "network"
	MemoryCategoryArchive   = "archive"
	MemoryCategoryRules     = "rules"
)

// VibeMemory stores structured memories for a workspace.
// Five categories: profile, knowledge, network, archive, rules.
// Free users: profile + rules only. Pro: all 5.
type VibeMemory struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WorkspaceID uuid.UUID      `gorm:"type:uuid;not null;index" json:"workspace_id"`
	Category    string         `gorm:"type:varchar(20);not null;index" json:"category"`
	Content     string         `gorm:"type:text;not null" json:"content"`
	Source      string         `gorm:"type:varchar(20);default:'user'" json:"source"` // user | ai | import
	SortOrder   int            `gorm:"default:0" json:"sort_order"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// VibeChat represents a conversation in a workspace.
type VibeChat struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WorkspaceID uuid.UUID      `gorm:"type:uuid;not null;index" json:"workspace_id"`
	UserID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"user_id"`
	Title       string         `gorm:"type:varchar(200)" json:"title"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// VibeChatMessage represents a message in a Vibe Write chat.
type VibeChatMessage struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ChatID      uuid.UUID `gorm:"type:uuid;not null;index" json:"chat_id"`
	Role        string    `gorm:"type:varchar(20);not null" json:"role"` // user | assistant
	Content     string    `gorm:"type:text;not null" json:"content"`
	CreditsCost int       `gorm:"default:0" json:"credits_cost"`
	UsedSoul    bool      `gorm:"default:false" json:"used_soul"`
	Model       string    `gorm:"type:varchar(50)" json:"model,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}
