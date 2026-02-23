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
	ID             uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Balance        float64   `gorm:"type:decimal(28,8);default:0" json:"balance"`          // Current $Ensoul in pool
	TotalDeposited float64   `gorm:"type:decimal(28,8);default:0" json:"total_deposited"`  // Cumulative deposits
	TotalReleased  float64   `gorm:"type:decimal(28,8);default:0" json:"total_released"`   // Cumulative releases
	DailyReleased  float64   `gorm:"type:decimal(28,8);default:0" json:"daily_released"`   // Released today
	LastResetAt    time.Time `json:"last_reset_at"`
	UpdatedAt      time.Time `json:"updated_at"`
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
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ClawID     uuid.UUID  `gorm:"type:uuid;not null;index" json:"claw_id"`
	FragmentID uuid.UUID  `gorm:"type:uuid;not null;index" json:"fragment_id"`
	DemandID   *uuid.UUID `gorm:"type:uuid;index" json:"demand_id,omitempty"`
	Amount     float64    `gorm:"type:decimal(18,8);not null" json:"amount"` // $Ensoul amount
	TxHash     string     `gorm:"type:varchar(66)" json:"tx_hash,omitempty"`
	Status     string     `gorm:"type:varchar(20);default:'pending'" json:"status"`
	CreatedAt  time.Time  `json:"created_at"`

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
// Soul Sniper Models (Phase 3)
// ═══════════════════════════════════════════════════════════════════════

// Subscription tier constants
const (
	SubTierStarter = "starter" // 3 KOLs, 10 replies/day, deepseek-v3
	SubTierPro     = "pro"     // 10 KOLs, 50 replies/day, gpt-4o
	SubTierElite   = "elite"   // 30 KOLs, unlimited, claude-opus
)

// Subscription status constants
const (
	SubStatusActive    = "active"
	SubStatusExpired   = "expired"
	SubStatusCancelled = "cancelled"
)

// Subscription represents a user's Soul Sniper subscription.
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

// SniperKOL represents a KOL that a subscriber is tracking.
type SniperKOL struct {
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

// SniperReply represents a generated reply for a KOL's tweet.
type SniperReply struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID    uuid.UUID `gorm:"type:uuid;not null;index" json:"shell_id"`
	WalletAddr string    `gorm:"type:varchar(42);not null;index" json:"wallet_addr"`
	TweetID    string    `gorm:"type:varchar(30);not null;index" json:"tweet_id"`
	TweetText  string    `gorm:"type:text" json:"tweet_text"`
	Replies    JSON      `gorm:"type:jsonb;default:'[]'" json:"replies"` // [{style, content, model}]
	CreatedAt  time.Time `json:"created_at"`

	// Relations
	Shell Shell `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
}

// UserPersona represents a user's custom persona for reply generation.
type UserPersona struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	WalletAddr string    `gorm:"type:varchar(42);uniqueIndex;not null" json:"wallet_addr"`
	Bio        string    `gorm:"type:text" json:"bio"`
	Style      string    `gorm:"type:text" json:"style"`
	Materials  string    `gorm:"type:text" json:"materials"` // reference materials
	Language   string    `gorm:"type:varchar(10);default:'en'" json:"language"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// SubscriptionTierConfig holds the limits for each subscription tier.
type SubscriptionTierConfig struct {
	MaxKOLs       int
	DailyReplies  int // -1 = unlimited
	DefaultModel  string
	MonthlyPriceUSDT float64
}

// SubscriptionTiers maps tier names to their configurations.
var SubscriptionTiers = map[string]SubscriptionTierConfig{
	SubTierStarter: {MaxKOLs: 3, DailyReplies: 10, DefaultModel: "deepseek-chat", MonthlyPriceUSDT: 9.9},
	SubTierPro:     {MaxKOLs: 10, DailyReplies: 50, DefaultModel: "gpt-4o", MonthlyPriceUSDT: 29.9},
	SubTierElite:   {MaxKOLs: 30, DailyReplies: -1, DefaultModel: "claude-sonnet-4-20250514", MonthlyPriceUSDT: 99.9},
}

// PublicSoul tracks Soul NFTs minted by the Tax Wallet as public assets.
type PublicSoul struct {
	ID        uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ShellID   uuid.UUID      `gorm:"type:uuid;not null;index" json:"shell_id"`
	MintCost  float64        `gorm:"type:decimal(18,8)" json:"mint_cost"`  // BNB spent
	SalePrice float64        `gorm:"type:decimal(18,8)" json:"sale_price"` // Listed price (with premium)
	Status    string         `gorm:"type:varchar(20);default:'minted'" json:"status"` // minted/listed/sold
	BuyerAddr string         `gorm:"type:varchar(42)" json:"buyer_addr,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`

	// Relations
	Shell Shell `gorm:"foreignKey:ShellID" json:"shell,omitempty"`
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
	TotalRevenue float64  `gorm:"type:decimal(28,8);default:0" json:"total_revenue"`
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
