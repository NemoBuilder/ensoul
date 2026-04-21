package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Shell stage constants
const (
	StagePending  = "pending" // Mint requested but not yet confirmed on-chain
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
	TwitterMeta   JSON           `gorm:"type:jsonb;default:'{}'" json:"twitter_meta"`
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
	Content      string         `gorm:"type:text;not null" json:"content,omitempty"`
	ContentHash  string         `gorm:"type:varchar(64);not null;default:''" json:"content_hash"`
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
	MiningApproved   bool           `gorm:"default:false" json:"mining_approved"`
	TwitterHandle    string         `gorm:"type:varchar(255)" json:"twitter_handle,omitempty"`
	TwitterTweetURL  string         `gorm:"type:text" json:"twitter_tweet_url,omitempty"`
	WalletAddr       string         `gorm:"type:varchar(42)" json:"wallet_addr"`
	WalletPKEnc      string         `gorm:"type:text" json:"-"`
	TotalSubmitted   int            `gorm:"default:0" json:"total_submitted"`
	TotalAccepted    int            `gorm:"default:0" json:"total_accepted"`
	Earnings         float64        `gorm:"type:decimal(18,8);default:0" json:"earnings"`
	Withdrawn        float64        `gorm:"type:decimal(18,8);default:0" json:"withdrawn"`
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
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TokenHash  string     `gorm:"column:token_hash;type:varchar(64);uniqueIndex;not null" json:"-"`
	WalletAddr string     `gorm:"type:varchar(42);not null;index" json:"wallet_addr"`
	UserID     *uuid.UUID `gorm:"type:uuid;index" json:"user_id,omitempty"`
	ExpiresAt  time.Time  `gorm:"not null" json:"expires_at"`
	CreatedAt  time.Time  `json:"created_at"`
}

// EmailSession stores email-based login sessions (HttpOnly cookie).
type EmailSession struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TokenHash string    `gorm:"column:token_hash;type:varchar(64);uniqueIndex;not null" json:"-"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Email     string    `gorm:"type:varchar(255);not null;index" json:"email"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
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
	UserID     *uuid.UUID     `gorm:"type:uuid;index" json:"user_id,omitempty"`
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

// ChatShare represents a publicly shareable snapshot of a conversation excerpt.
type ChatShare struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Code      string    `gorm:"type:varchar(16);uniqueIndex;not null" json:"code"`
	SessionID uuid.UUID `gorm:"type:uuid;not null;index" json:"session_id"`
	ShellID   uuid.UUID `gorm:"type:uuid;not null;index" json:"shell_id"`
	Handle    string    `gorm:"type:varchar(255);not null" json:"handle"`
	AvatarURL string    `gorm:"type:text" json:"avatar_url"`
	Stage     string    `gorm:"type:varchar(20)" json:"stage"`
	DNAVer    int       `gorm:"default:0" json:"dna_version"`
	Messages  string    `gorm:"type:text;not null" json:"messages"` // JSON array of [{role, content}]
	CreatedAt time.Time `json:"created_at"`
}

// ═══════════════════════════════════════════════════════════════════════
// Economic System Models (Ensoul-Next)
// ═══════════════════════════════════════════════════════════════════════

// Fragment demand status constants
const (
	DemandStatusOpen      = "open"
	DemandStatusFulfilled = "fulfilled"
	DemandStatusExpired   = "expired"
)

// Mining reward status constants
const (
	RewardStatusPending   = "pending"
	RewardStatusSent      = "sent"
	RewardStatusConfirmed = "confirmed"
	RewardStatusFailed    = "failed"
)

// MiningPool tracks the global mining pool state (single row).
type MiningPool struct {
	ID                uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Balance           float64   `gorm:"type:decimal(28,8);default:0" json:"balance"`             // Current $Ensoul in pool
	TotalDeposited    float64   `gorm:"type:decimal(28,8);default:0" json:"total_deposited"`     // Cumulative deposits
	TotalReleased     float64   `gorm:"type:decimal(28,8);default:0" json:"total_released"`      // Cumulative releases
	DailyReleased     float64   `gorm:"type:decimal(28,8);default:0" json:"daily_released"`      // Released today
	DailyStartBalance float64   `gorm:"type:decimal(28,8);default:0" json:"daily_start_balance"` // Balance snapshot at daily reset
	LastResetAt       time.Time `json:"last_reset_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// FragmentDemand represents a Crab-published fragment demand with bounty.
type FragmentDemand struct {
	ID          uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID     uuid.UUID      `gorm:"type:uuid;not null;index" json:"shell_id"`
	Dimension   string         `gorm:"type:varchar(20);not null" json:"dimension"`
	Description string         `gorm:"type:text" json:"description"`
	Bounty      float64        `gorm:"type:decimal(18,8);not null" json:"bounty"` // $Ensoul bounty
	Status      string         `gorm:"type:varchar(20);default:'open'" json:"status"`
	CreatedAt   time.Time      `json:"created_at"`
	ExpiresAt   time.Time      `json:"expires_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Shell Shell `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
}

// MiningReward records a reward paid to a Claw for a fragment contribution.
type MiningReward struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ClawID        uuid.UUID  `gorm:"type:uuid;not null;index" json:"claw_id"`
	FragmentID    uuid.UUID  `gorm:"type:uuid;not null;index" json:"fragment_id"`
	DemandID      *uuid.UUID `gorm:"type:uuid;index" json:"demand_id,omitempty"`
	Amount        float64    `gorm:"type:decimal(18,8);not null" json:"amount"` // $Ensoul amount
	TxHash        string     `gorm:"type:varchar(66)" json:"tx_hash,omitempty"`
	Status        string     `gorm:"type:varchar(20);default:'pending'" json:"status"`
	RetryCount    int        `gorm:"default:0" json:"retry_count"`
	LastError     string     `gorm:"type:text" json:"last_error,omitempty"`
	LastAttemptAt *time.Time `json:"last_attempt_at,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`

	// Relations
	Claw     Claw     `gorm:"foreignKey:ClawID" json:"claw,omitempty"`
	Fragment Fragment `gorm:"foreignKey:FragmentID" json:"fragment,omitempty"`
}

// BuybackRecord tracks each BNB → $Ensoul buyback operation.
type BuybackRecord struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Source      string    `gorm:"type:varchar(30);not null" json:"source"` // mint_revenue / subscription_revenue
	BNBAmount   float64   `gorm:"type:decimal(18,8)" json:"bnb_amount"`
	TokenAmount float64   `gorm:"type:decimal(28,8)" json:"token_amount"` // $Ensoul received
	SwapTxHash  string    `gorm:"type:varchar(66)" json:"swap_tx_hash"`
	CreatedAt   time.Time `json:"created_at"`
}

// ═══════════════════════════════════════════════════════════════════════
// Vibe Write Models (Phase 3)
// ═══════════════════════════════════════════════════════════════════════

// Subscription tier constants
const (
	SubTierFree = "free" // browse only, no snipe
	SubTierPro  = "pro"  // 50 snipes/day, Claude Sonnet
)

// Subscription status constants
const (
	SubStatusActive    = "active"
	SubStatusExpired   = "expired"
	SubStatusCancelled = "cancelled"
)

// Subscription represents a user's Vibe Write subscription.
//
// Deprecated: removed in vN+1, do not use. Subscription state now lives on
// User (ProExpiresAt + LemonSubscriptionID). Table is kept for one release
// to allow rollback; AutoMigrate still runs but no code path writes here.
type Subscription struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WalletAddr    string         `gorm:"type:varchar(42);not null;index" json:"wallet_addr"`
	Tier          string         `gorm:"type:varchar(20);not null" json:"tier"`
	LLMModel      string         `gorm:"type:varchar(50)" json:"llm_model"`
	Status        string         `gorm:"type:varchar(20);default:'active'" json:"status"`
	ExpiresAt     time.Time      `gorm:"not null" json:"expires_at"`
	PaymentTxHash string         `gorm:"type:varchar(66)" json:"payment_tx_hash"`
	PaymentToken  string         `gorm:"type:varchar(10);default:'USDT'" json:"payment_token"` // USDT/BNB/ENSOUL
	PaymentAmount float64        `gorm:"type:decimal(18,8)" json:"payment_amount"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// VibeWriteKOL represents a KOL that a subscriber is tracking.
//
// Deprecated: removed in vN+1, do not use. Replaced by VibeMemory (network category).
type VibeWriteKOL struct {
	ID             uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	SubscriptionID uuid.UUID      `gorm:"type:uuid;not null;index" json:"subscription_id"`
	ShellID        uuid.UUID      `gorm:"type:uuid;not null;index" json:"shell_id"`
	Handle         string         `gorm:"type:varchar(15);not null" json:"handle"`
	CreatedAt      time.Time      `json:"created_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Subscription Subscription `gorm:"foreignKey:SubscriptionID" json:"subscription,omitempty"`
	Shell        Shell        `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
}

// VibeWriteReply represents a generated reply for a tweet (Vibe Write 2.0).
//
// Deprecated: removed in vN+1, do not use. Replaced by VibeChatMessage with
// reply variants stored as JSON in Content.
type VibeWriteReply struct {
	ID           uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID      *uuid.UUID `gorm:"type:uuid;index" json:"shell_id"` // nullable: Soul is optional
	WalletAddr   string     `gorm:"type:varchar(42);not null;index" json:"wallet_addr"`
	TweetID      string     `gorm:"type:varchar(30);not null;index" json:"tweet_id"`
	TweetText    string     `gorm:"type:text" json:"tweet_text"`
	Replies      JSON       `gorm:"type:jsonb;default:'[]'" json:"replies"` // [{style, content, model}]
	AuthorHandle string     `gorm:"type:varchar(30)" json:"author_handle"`  // tweet author handle
	TagID        string     `gorm:"type:varchar(50)" json:"tag_id"`         // originating tag
	TweetURL     string     `gorm:"type:text" json:"tweet_url"`             // full tweet URL
	UsedSoul     bool       `gorm:"default:false" json:"used_soul"`         // whether Soul persona was used
	CreatedAt    time.Time  `json:"created_at"`

	// Relations
	Shell *Shell `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
}

// UserPersona represents a user's custom persona for reply generation.
//
// Deprecated: removed in vN+1, do not use. Replaced by VibeMemory (profile + rules categories).
type UserPersona struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WalletAddr string    `gorm:"type:varchar(42);uniqueIndex;not null" json:"wallet_addr"`
	Bio        string    `gorm:"type:text" json:"bio"`
	Style      string    `gorm:"type:text" json:"style"`
	Materials  string    `gorm:"type:text" json:"materials"` // reference materials
	Language   string    `gorm:"type:varchar(10);default:'en'" json:"language"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// ═══════════════════════════════════════════════════════════════════════
// Vibe Write 2.0: Tag-based Feed + Snipe Models
// ═══════════════════════════════════════════════════════════════════════

// VibeWriteTag defines a content tag (maintained by Admin).
//
// Deprecated: removed in vN+1, do not use. Tag-based feed was removed in V3.
type VibeWriteTag struct {
	ID          string    `gorm:"type:varchar(50);primaryKey" json:"id"` // e.g. "bnb_official"
	Name        string    `gorm:"type:varchar(100)" json:"name"`         // Chinese name
	NameEN      string    `gorm:"type:varchar(100)" json:"name_en"`      // English name
	Icon        string    `gorm:"type:varchar(10)" json:"icon"`          // Emoji icon
	Category    string    `gorm:"type:varchar(20)" json:"category"`      // ecosystem / track / custom
	Description string    `gorm:"type:text" json:"description"`
	IsDefault   bool      `gorm:"default:false" json:"is_default"` // auto-selected for new users
	Active      bool      `gorm:"default:true" json:"active"`
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// VibeWriteTagAccount links a Twitter account to a tag (many-to-many, Admin maintained).
//
// Deprecated: removed in vN+1, do not use. Tag-based feed was removed in V3.
type VibeWriteTagAccount struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TagID            string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_tag_account" json:"tag_id"`
	Handle           string    `gorm:"type:varchar(30);not null;uniqueIndex:idx_tag_account" json:"handle"`
	DisplayName      string    `gorm:"type:varchar(100)" json:"display_name"`  // cached display name
	RealtimePriority bool      `gorm:"default:false" json:"realtime_priority"` // allocate to Twitter Stream
	SortOrder        int       `gorm:"default:0" json:"sort_order"`
	CreatedAt        time.Time `json:"created_at"`
}

// TagCandidate represents an AI-recommended account pending admin review.
//
// Deprecated: removed in vN+1, do not use. Tag-based feed was removed in V3.
type TagCandidate struct {
	ID               uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Handle           string         `gorm:"type:varchar(30);not null;index" json:"handle"`
	DisplayName      string         `gorm:"type:varchar(100)" json:"display_name"`
	Bio              string         `gorm:"type:text" json:"bio"`
	FollowersCount   int            `gorm:"default:0" json:"followers_count"`
	Source           string         `gorm:"type:varchar(20)" json:"source"`                         // "list_import" | "batch_input"
	SourceDetail     string         `gorm:"type:varchar(255)" json:"source_detail"`                 // List URL or batch note
	RecommendedTags  JSON           `gorm:"type:jsonb" json:"recommended_tags"`                     // [{"id":"bnb_official","confidence":0.85}]
	AIReason         string         `gorm:"type:text" json:"ai_reason"`                             // LLM recommendation reason
	Status           string         `gorm:"type:varchar(20);default:'pending';index" json:"status"` // pending | approved | rejected
	ApprovedTags     JSON           `gorm:"type:jsonb" json:"approved_tags"`                        // Admin-confirmed tag IDs
	RealtimePriority bool           `gorm:"default:false" json:"realtime_priority"`
	ReviewedBy       string         `gorm:"type:varchar(42)" json:"reviewed_by"` // Admin wallet
	ReviewedAt       *time.Time     `json:"reviewed_at"`
	CreatedAt        time.Time      `json:"created_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// TagCandidate status constants
const (
	TagCandidatePending  = "pending"
	TagCandidateApproved = "approved"
	TagCandidateRejected = "rejected"
)

// UserSelectedTag records which tags a user has selected for their feed.
//
// Deprecated: removed in vN+1, do not use. Tag-based feed was removed in V3.
type UserSelectedTag struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WalletAddr string    `gorm:"type:varchar(42);not null;uniqueIndex:idx_user_tags" json:"wallet_addr"`
	TagID      string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_user_tags" json:"tag_id"`
	CreatedAt  time.Time `json:"created_at"`
}

// UserMutedAccount records accounts a user has muted from their feed.
//
// Deprecated: removed in vN+1, do not use. Tag-based feed was removed in V3.
type UserMutedAccount struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WalletAddr string    `gorm:"type:varchar(42);not null;uniqueIndex:idx_user_muted" json:"wallet_addr"`
	Handle     string    `gorm:"type:varchar(30);not null;uniqueIndex:idx_user_muted" json:"handle"`
	CreatedAt  time.Time `json:"created_at"`
}

// ═══════════════════════════════════════════════════════════════════════
// Vibe Write 2.0+: Multi-Dimensional Tagging
// ═══════════════════════════════════════════════════════════════════════

// TagDimension defines a tagging axis (e.g. "chain", "track", "role").
//
// Deprecated: removed in vN+1, do not use. Multi-dimensional tagging was removed in V3.
type TagDimension struct {
	ID        string    `gorm:"type:varchar(30);primaryKey" json:"id"` // e.g. "chain"
	Name      string    `gorm:"type:varchar(50)" json:"name"`          // Chinese name
	NameEN    string    `gorm:"type:varchar(50)" json:"name_en"`       // English name
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	Active    bool      `gorm:"default:true" json:"active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TagDimensionValue is a specific value within a dimension (e.g. "chain:bnb").
//
// Deprecated: removed in vN+1, do not use. Multi-dimensional tagging was removed in V3.
type TagDimensionValue struct {
	ID          string    `gorm:"type:varchar(50);primaryKey" json:"id"` // e.g. "chain:bnb"
	DimensionID string    `gorm:"type:varchar(30);not null;index" json:"dimension_id"`
	Label       string    `gorm:"type:varchar(50)" json:"label"`    // Chinese label
	LabelEN     string    `gorm:"type:varchar(50)" json:"label_en"` // English label
	Icon        string    `gorm:"type:varchar(10)" json:"icon"`     // Emoji icon
	SortOrder   int       `gorm:"default:0" json:"sort_order"`
	Active      bool      `gorm:"default:true" json:"active"`
	CreatedAt   time.Time `json:"created_at"`
}

// VibeWriteTagDimension links a VibeWriteTag to one or more dimension values (many-to-many).
//
// Deprecated: removed in vN+1, do not use. Multi-dimensional tagging was removed in V3.
type VibeWriteTagDimension struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TagID            string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_tag_dim_val" json:"tag_id"`
	DimensionValueID string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_tag_dim_val" json:"dimension_value_id"`
	CreatedAt        time.Time `json:"created_at"`
}

// ExternalSnipeUsage tracks daily usage for external snipe API callers.
//
// Deprecated: removed in vN+1, do not use. /snipe endpoint was removed in V3.
type ExternalSnipeUsage struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	CallerID  string    `gorm:"type:varchar(100);not null;index" json:"caller_id"`
	Date      string    `gorm:"type:varchar(10);not null;index" json:"date"` // YYYY-MM-DD
	Count     int       `gorm:"default:0" json:"count"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SubscriptionTierConfig holds the limits for each subscription tier.
type SubscriptionTierConfig struct {
	DailyReplies     int // 0 = not available, 50 = daily snipe limit
	DefaultModel     string
	MonthlyPriceUSDT float64
}

// SubscriptionTiers maps tier names to their configurations.
// Free users have no Subscription record (nil = free tier).
// Pro users have an active Subscription record with tier = "pro".
var SubscriptionTiers = map[string]SubscriptionTierConfig{
	SubTierFree: {DailyReplies: 0, DefaultModel: "", MonthlyPriceUSDT: 0},
	SubTierPro:  {DailyReplies: 50, DefaultModel: "claude-sonnet-4-20250514", MonthlyPriceUSDT: 99},
}

// PublicSoul tracks Soul NFTs minted by the Tax Wallet as public assets.
type PublicSoul struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"shell_id"`
	MintCost  float64        `gorm:"type:decimal(18,8)" json:"mint_cost"`             // BNB spent
	SalePrice float64        `gorm:"type:decimal(18,8)" json:"sale_price"`            // Listed price (with premium)
	Status    string         `gorm:"type:varchar(20);default:'minted'" json:"status"` // minted/listed/sold
	BuyerAddr string         `gorm:"type:varchar(42)" json:"buyer_addr,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Shell Shell `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
}

// MintCandidate represents a Twitter handle queued for public Soul minting by the Tax Wallet.
// Managed by admin via API. The 30-second scheduler picks "pending" candidates.
type MintCandidate struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Handle    string         `gorm:"type:varchar(30);uniqueIndex;not null" json:"handle"`
	Followers int            `gorm:"default:0" json:"followers"`                       // fetched when added
	PriceWei  string         `gorm:"type:varchar(78);default:'0'" json:"price_wei"`    // mint price in wei (stored as string)
	Tier      string         `gorm:"type:varchar(20)" json:"tier"`                     // micro/small/medium/large/top/super
	Priority  int            `gorm:"default:0" json:"priority"`                        // higher = mint first
	Reason    string         `gorm:"type:varchar(200)" json:"reason"`                  // why this handle was added
	Status    string         `gorm:"type:varchar(20);default:'pending'" json:"status"` // pending/minted/skipped/failed
	ErrorMsg  string         `gorm:"type:text" json:"error_msg,omitempty"`
	AddedBy   string         `gorm:"type:varchar(42)" json:"added_by"` // admin identifier
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// MintCandidate status constants
const (
	CandidateStatusPending = "pending" // auto-mint scheduler picks these up
	CandidateStatusQueued  = "queued"  // waiting for manual mint (batch/import)
	CandidateStatusMinted  = "minted"
	CandidateStatusSkipped = "skipped"
	CandidateStatusFailed  = "failed"
)

// ═══════════════════════════════════════════════════════════════════════
// Admin Authentication Models
// ═══════════════════════════════════════════════════════════════════════

// AdminRole constants
const (
	AdminRoleSuperAdmin = "super_admin"
	AdminRoleOperator   = "operator"
)

// AdminUser represents an admin account with username/password login.
type AdminUser struct {
	ID           uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Username     string         `gorm:"type:varchar(50);uniqueIndex;not null" json:"username"`
	PasswordHash string         `gorm:"type:varchar(100);not null" json:"-"` // bcrypt hash
	Role         string         `gorm:"type:varchar(20);default:'operator'" json:"role"`
	LastLoginAt  *time.Time     `json:"last_login_at,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// AdminSession represents an authenticated admin session (HttpOnly cookie).
type AdminSession struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TokenHash   string    `gorm:"column:token_hash;type:varchar(64);uniqueIndex;not null" json:"-"`
	AdminUserID uuid.UUID `gorm:"type:uuid;not null;index" json:"admin_user_id"`
	ExpiresAt   time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt   time.Time `json:"created_at"`

	// Relations
	AdminUser AdminUser `gorm:"foreignKey:AdminUserID" json:"admin_user,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════
// Holder Revenue & KOL Claim Models (Phase 4)
// ═══════════════════════════════════════════════════════════════════════

// Holder revenue status constants
const (
	HolderRevenueStatusPending   = "pending"
	HolderRevenueStatusSent      = "sent"
	HolderRevenueStatusConfirmed = "confirmed"
	HolderRevenueStatusClaimed   = "claimed"
)

// KOL claim status constants
const (
	ClaimStatusPending  = "pending"
	ClaimStatusVerified = "verified"
	ClaimStatusRejected = "rejected"
)

// HolderRevenue records a monthly revenue share for a Soul holder.
type HolderRevenue struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID    uuid.UUID `gorm:"type:uuid;not null;index" json:"shell_id"`
	WalletAddr string    `gorm:"type:varchar(42);not null;index" json:"wallet_addr"`
	Period     string    `gorm:"type:varchar(7);not null;index" json:"period"` // "2026-02"
	UsageCount int       `gorm:"default:0" json:"usage_count"`
	Weight     float64   `gorm:"type:decimal(18,8);default:0" json:"weight"`
	Amount     float64   `gorm:"type:decimal(18,8);default:0" json:"amount"` // $Ensoul
	TxHash     string    `gorm:"type:varchar(66)" json:"tx_hash,omitempty"`
	Status     string    `gorm:"type:varchar(20);default:'pending'" json:"status"`
	CreatedAt  time.Time `json:"created_at"`

	// Relations
	Shell Shell `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
}

// RevenuePool tracks the monthly revenue pool state.
type RevenuePool struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Period       string    `gorm:"type:varchar(7);uniqueIndex;not null" json:"period"` // "2026-02"
	TotalRevenue float64   `gorm:"type:decimal(28,8);default:0" json:"total_revenue"`
	PoolAmount   float64   `gorm:"type:decimal(28,8);default:0" json:"pool_amount"` // 15% of total
	Distributed  bool      `gorm:"default:false" json:"distributed"`
	CreatedAt    time.Time `json:"created_at"`
}

// KOLClaim records a KOL's claim request for their Soul.
type KOLClaim struct {
	ID            uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID       uuid.UUID  `gorm:"type:uuid;not null;uniqueIndex" json:"shell_id"`
	KOLWalletAddr string     `gorm:"type:varchar(42);not null" json:"kol_wallet_addr"`
	VerifyCode    string     `gorm:"type:varchar(20);not null" json:"verify_code"`
	VerifyTweetID string     `gorm:"type:varchar(30)" json:"verify_tweet_id"`
	Status        string     `gorm:"type:varchar(20);default:'pending'" json:"status"`
	ClaimedAt     *time.Time `json:"claimed_at,omitempty"`
	TransitionEnd *time.Time `json:"transition_end,omitempty"` // +3 months after claim
	CreatedAt     time.Time  `json:"created_at"`

	// Relations
	Shell Shell `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
}

// SoulUsage tracks monthly usage counts per Soul.
type SoulUsage struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID    uuid.UUID `gorm:"type:uuid;not null;index:idx_soul_usage_period" json:"shell_id"`
	Period     string    `gorm:"type:varchar(7);not null;index:idx_soul_usage_period" json:"period"` // "2026-02"
	UsageCount int       `gorm:"default:0" json:"usage_count"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// UsedPaymentTx prevents replay attacks by recording each payment tx_hash.
// The uniqueIndex on TxHash ensures the same transaction cannot be used twice.
type UsedPaymentTx struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TxHash     string    `gorm:"type:varchar(66);not null;uniqueIndex" json:"tx_hash"`
	WalletAddr string    `gorm:"type:varchar(42);not null" json:"wallet_addr"`
	Purpose    string    `gorm:"type:varchar(30);not null" json:"purpose"` // "subscription", "mint", etc.
	CreatedAt  time.Time `json:"created_at"`
}

// Withdraw status constants
const (
	WithdrawStatusPending   = "pending"
	WithdrawStatusSent      = "sent"
	WithdrawStatusConfirmed = "confirmed"
	WithdrawStatusFailed    = "failed"
)

// WithdrawRecord tracks a withdrawal from a Claw wallet to a user wallet.
type WithdrawRecord struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ClawID    uuid.UUID `gorm:"type:uuid;not null;index" json:"claw_id"`
	FromAddr  string    `gorm:"type:varchar(42);not null" json:"from_addr"` // Claw wallet
	ToAddr    string    `gorm:"type:varchar(42);not null" json:"to_addr"`   // User wallet
	Amount    float64   `gorm:"type:decimal(18,8);not null" json:"amount"`  // $Ensoul
	TxHash    string    `gorm:"type:varchar(66)" json:"tx_hash,omitempty"`
	Status    string    `gorm:"type:varchar(20);default:'pending'" json:"status"`
	LastError string    `gorm:"type:text" json:"last_error,omitempty"`
	CreatedAt time.Time `json:"created_at"`

	// Relations
	Claw Claw `gorm:"foreignKey:ClawID" json:"claw,omitempty"`
}

// ═══════════════════════════════════════════════════════════════════════
// User Management Models
// ═══════════════════════════════════════════════════════════════════════

// User status constants
const (
	UserStatusActive = "active"
	UserStatusBanned = "banned"
)

// User represents a registered user account.
// Can be created via email signup or wallet login.
type User struct {
	ID            uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email         string         `gorm:"type:varchar(255);uniqueIndex" json:"email,omitempty"`
	EmailVerified bool           `gorm:"default:false" json:"email_verified"`
	PasswordHash  string         `gorm:"type:varchar(255)" json:"-"`
	WalletAddr    string         `gorm:"type:varchar(42);uniqueIndex" json:"wallet_addr,omitempty"`
	TwitterHandle string         `gorm:"type:varchar(30)" json:"twitter_handle,omitempty"`
	Status        string         `gorm:"type:varchar(20);default:'active'" json:"status"`
	BanReason     string         `gorm:"type:text" json:"ban_reason,omitempty"`
	BannedAt      *time.Time     `json:"banned_at,omitempty"`
	BannedBy      string         `gorm:"type:varchar(50)" json:"banned_by,omitempty"`
	Note          string         `gorm:"type:text" json:"note,omitempty"`
	ProExpiresAt        *time.Time     `json:"pro_expires_at,omitempty"`
	LemonSubscriptionID string         `gorm:"type:varchar(100)" json:"lemon_subscription_id,omitempty"`
	Credits             int            `gorm:"default:50" json:"credits"`
	CreditsReset  time.Time      `json:"credits_reset"`
	FirstSeenAt   time.Time      `json:"first_seen_at"`
	LastSeenAt    time.Time      `json:"last_seen_at"`
	LoginCount    int            `gorm:"default:0" json:"login_count"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// IsPro returns whether the user currently has an active Pro subscription.
func (u *User) IsPro() bool {
	return u.ProExpiresAt != nil && u.ProExpiresAt.After(time.Now())
}

// EmailCode stores email verification codes with expiry.
type EmailCode struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Email     string    `gorm:"type:varchar(255);not null;index" json:"email"`
	Code      string    `gorm:"type:varchar(6);not null" json:"-"`
	Used      bool      `gorm:"default:false" json:"used"`
	ExpiresAt time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// AdminAuditLog records admin operations for auditing.
type AdminAuditLog struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	AdminUserID uuid.UUID `gorm:"type:uuid;not null;index" json:"admin_user_id"`
	AdminName   string    `gorm:"type:varchar(50)" json:"admin_name"`
	Action      string    `gorm:"type:varchar(50);not null;index" json:"action"`
	TargetType  string    `gorm:"type:varchar(30)" json:"target_type"`
	TargetID    string    `gorm:"type:varchar(100)" json:"target_id"`
	Detail      JSON      `gorm:"type:jsonb;default:'{}'" json:"detail"`
	IP          string    `gorm:"type:varchar(45)" json:"ip"`
	CreatedAt   time.Time `json:"created_at"`
}
