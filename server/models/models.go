package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Shell stage constants
const (
	StageEmbryo   = "embryo"
	StageGrowing  = "growing"
	StageMature   = "mature"
	StageEvolving = "evolving"
)

// Fragment dimension constants
const (
	DimPersonality  = "personality"
	DimKnowledge    = "knowledge"
	DimStance       = "stance"
	DimStyle        = "style"
	DimRelationship = "relationship"
	DimTimeline     = "timeline"
)

// Fragment status constants
const (
	FragStatusPending  = "pending"
	FragStatusAccepted = "accepted"
	FragStatusRejected = "rejected"
)

// Claw status constants
const (
	ClawStatusPendingClaim = "pending_claim"
	ClawStatusClaimed      = "claimed"
)

// Shell represents a Soul / DNA NFT on-chain.
type Shell struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Handle        string         `gorm:"uniqueIndex;not null" json:"handle"`
	TokenID       *uint64        `gorm:"type:bigint" json:"token_id"`
	OwnerAddr     string         `gorm:"type:varchar(42)" json:"owner_addr"`
	Stage         string         `gorm:"type:varchar(20);default:'embryo'" json:"stage"`
	DNAVersion    int            `gorm:"default:0" json:"dna_version"`
	SeedSummary   string         `gorm:"type:text" json:"seed_summary"`
	SoulPrompt    string         `gorm:"type:text" json:"soul_prompt"`
	Dimensions    JSON           `gorm:"type:jsonb;default:'{}'" json:"dimensions"`
	TotalFrags    int            `gorm:"default:0" json:"total_frags"`
	AcceptedFrags int            `gorm:"default:0" json:"accepted_frags"`
	TotalClaws    int            `gorm:"default:0" json:"total_claws"`
	TotalChats    int            `gorm:"default:0" json:"total_chats"`
	AvatarURL     string         `gorm:"type:text" json:"avatar_url"`
	DisplayName   string         `gorm:"type:varchar(255)" json:"display_name"`
	AgentID       *uint64        `gorm:"type:bigint" json:"agent_id"` // ERC-8004 agent ID
	AgentURI      string         `gorm:"type:text" json:"agent_uri"`
	MintTxHash    string         `gorm:"type:varchar(66)" json:"mint_tx_hash,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// Fragment represents a piece of soul data contributed by a Claw.
type Fragment struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID      uuid.UUID      `gorm:"type:uuid;not null;index" json:"shell_id"`
	ClawID       uuid.UUID      `gorm:"type:uuid;not null;index" json:"claw_id"`
	Dimension    string         `gorm:"type:varchar(20);not null" json:"dimension"`
	Content      string         `gorm:"type:text;not null" json:"content"`
	Status       string         `gorm:"type:varchar(20);default:'pending'" json:"status"`
	Confidence   float64        `gorm:"type:decimal(3,2);default:0" json:"confidence"`
	RejectReason string         `gorm:"type:text" json:"reject_reason,omitempty"`
	EnsoulingID  *uuid.UUID     `gorm:"type:uuid" json:"ensouling_id,omitempty"`
	TxHash       string         `gorm:"type:varchar(66)" json:"tx_hash,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Shell Shell `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
	Claw  Claw  `gorm:"foreignKey:ClawID" json:"claw,omitempty"`
}

// Claw represents an AI agent that contributes fragments.
type Claw struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name             string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"name"`
	Description      string         `gorm:"type:text" json:"description"`
	APIKeyHash       string         `gorm:"column:api_key_hash;type:varchar(64);uniqueIndex;not null" json:"-"`
	ClaimCode        string         `gorm:"type:varchar(255);uniqueIndex;not null" json:"-"`
	VerificationCode string         `gorm:"type:varchar(20);not null" json:"-"`
	Status           string         `gorm:"type:varchar(20);default:'pending_claim'" json:"status"`
	TwitterHandle    string         `gorm:"type:varchar(255)" json:"twitter_handle,omitempty"`
	TwitterTweetURL  string         `gorm:"type:text" json:"twitter_tweet_url,omitempty"`
	WalletAddr       string         `gorm:"type:varchar(42)" json:"wallet_addr"`
	WalletPKEnc      string         `gorm:"type:text" json:"-"`
	TotalSubmitted   int            `gorm:"default:0" json:"total_submitted"`
	TotalAccepted    int            `gorm:"default:0" json:"total_accepted"`
	Earnings         float64        `gorm:"type:decimal(18,8);default:0" json:"earnings"`
	CreatedAt        time.Time      `json:"created_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// Ensouling represents a soul condensation event.
type Ensouling struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID     uuid.UUID `gorm:"type:uuid;not null;index" json:"shell_id"`
	VersionFrom int       `gorm:"not null" json:"version_from"`
	VersionTo   int       `gorm:"not null" json:"version_to"`
	FragsMerged int       `gorm:"not null" json:"frags_merged"`
	SummaryDiff string    `gorm:"type:text" json:"summary_diff"`
	NewPrompt   string    `gorm:"type:text" json:"new_prompt"`
	TxHash      string    `gorm:"type:varchar(66)" json:"tx_hash,omitempty"`
	CreatedAt   time.Time `json:"created_at"`

	// Relations
	Shell Shell `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
}

// WalletSession represents an authenticated wallet session (HttpOnly cookie).
type WalletSession struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TokenHash  string    `gorm:"column:token_hash;type:varchar(64);uniqueIndex;not null" json:"-"`
	WalletAddr string    `gorm:"type:varchar(42);not null;index" json:"wallet_addr"`
	ExpiresAt  time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt  time.Time `json:"created_at"`
}

// ClawBinding binds a Claw API key to a wallet address.
type ClawBinding struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WalletAddr string    `gorm:"type:varchar(42);not null;index" json:"wallet_addr"`
	ClawID     uuid.UUID `gorm:"type:uuid;not null;index" json:"claw_id"`
	ClawName   string    `gorm:"type:varchar(255)" json:"claw_name"`
	CreatedAt  time.Time `json:"created_at"`

	// Relations
	Claw Claw `gorm:"foreignKey:ClawID" json:"claw,omitempty"`
}

// Chat tier constants
const (
	ChatTierGuest = "guest" // Anonymous user, limited rounds
	ChatTierFree  = "free"  // Logged-in user, unlimited rounds
	ChatTierPaid  = "paid"  // Future: paid access with extended context
)

// Chat round limits per tier
const (
	ChatGuestMaxRounds = 5
)

// ChatSession represents a conversation session with a soul.
type ChatSession struct {
	ID         uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID    uuid.UUID      `gorm:"type:uuid;not null;index" json:"shell_id"`
	WalletAddr string         `gorm:"type:varchar(42);index" json:"wallet_addr,omitempty"` // empty = guest
	Tier       string         `gorm:"type:varchar(20);default:'guest'" json:"tier"`
	Rounds     int            `gorm:"default:0" json:"rounds"` // number of user messages sent
	Title      string         `gorm:"type:varchar(255)" json:"title,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Shell    Shell         `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
	Messages []ChatMessage `gorm:"foreignKey:SessionID" json:"messages,omitempty"`
}

// ChatMessage represents a single message in a chat session.
type ChatMessage struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SessionID uuid.UUID `gorm:"type:uuid;not null;index" json:"session_id"`
	Role      string    `gorm:"type:varchar(20);not null" json:"role"` // "user" or "assistant"
	Content   string    `gorm:"type:text;not null" json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
