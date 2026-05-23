// Package models — V4 Galaxy data model.
//
// Galaxy is the V4 evolution of Soul: any topic (person, project, discipline,
// place, event, …) can become its own knowledge galaxy that grows atom-by-atom
// via community contributions, distilled by LLM, and (when mature) launches a
// community token under a fair-launch protocol.
//
// This file is V4-only. V3 modules (Shell / Fragment / Claw / Mining) keep
// their own files. V4 reuses V3's User / WalletSession / EmailSession / Crypto
// payment skeleton; do not duplicate auth or billing here.
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ─── Galaxy lifecycle ────────────────────────────────────────────────────────

const (
	// GalaxyStageApplying — application submitted, awaiting curator review.
	GalaxyStageApplying = "applying"
	// GalaxyStageEmbryo — approved, taking initial atoms.
	GalaxyStageEmbryo = "embryo"
	// GalaxyStageGrowing — active contribution, growing graph.
	GalaxyStageGrowing = "growing"
	// GalaxyStageMature — passed LaunchReady gates, eligible for fair launch.
	GalaxyStageMature = "mature"
	// GalaxyStageRaising — fair-launch window open (e.g. 72h).
	GalaxyStageRaising = "raising"
	// GalaxyStageGraduated — fair launch succeeded, community token live.
	GalaxyStageGraduated = "graduated"
	// GalaxyStageRejected — application denied / quality failed.
	GalaxyStageRejected = "rejected"
)

// ─── Atom status ─────────────────────────────────────────────────────────────

const (
	AtomStatusPending  = "pending"  // queued for distillation
	AtomStatusDistilling = "distilling"
	AtomStatusAccepted = "accepted" // merged into graph
	AtomStatusRejected = "rejected" // failed intake or alignment
	AtomStatusDisputed = "disputed" // under curator review
)

// ─── Edge directionality ─────────────────────────────────────────────────────

const (
	EdgeDirected   = "directed"
	EdgeUndirected = "undirected"
)

// Galaxy is the core V4 entity — a knowledge graph about one subject.
type Galaxy struct {
	ID           uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Slug         string    `gorm:"type:varchar(64);uniqueIndex;not null" json:"slug"` // url-friendly handle
	Title        string    `gorm:"type:varchar(255);not null" json:"title"`
	Subtitle     string    `gorm:"type:text" json:"subtitle,omitempty"`
	CoverURL     string    `gorm:"type:text" json:"cover_url,omitempty"`
	Category     string    `gorm:"type:varchar(64);index" json:"category,omitempty"` // person / project / discipline / place / event / other
	Lang         string    `gorm:"type:varchar(8);default:'en'" json:"lang"`

	// Founder = the user who applied to create this galaxy.
	// Roles (Contributor / Backer / Curator) are tracked elsewhere via
	// GalaxyRole rows. All roles point to existing models.User.
	FounderID    uuid.UUID `gorm:"type:uuid;not null;index" json:"founder_id"`

	Stage        string    `gorm:"type:varchar(20);default:'applying';index" json:"stage"`

	// Aggregated counters (denormalised; recomputed on accept/reject).
	AtomCount    int       `gorm:"default:0" json:"atom_count"`
	EdgeCount    int       `gorm:"default:0" json:"edge_count"`
	NodeCount    int       `gorm:"default:0" json:"node_count"`
	ContribCount int       `gorm:"default:0" json:"contrib_count"`

	// LaunchReady gate signals (rolled up from quality.go). Stored so the
	// listing page can sort/filter cheaply without joining the graph.
	MaturityScore   float64 `gorm:"type:decimal(5,2);default:0" json:"maturity_score"`     // 0..100
	DiversityScore  float64 `gorm:"type:decimal(5,2);default:0" json:"diversity_score"`    // 0..100
	ConfidenceAvg   float64 `gorm:"type:decimal(4,3);default:0" json:"confidence_avg"`     // 0..1
	AntiFarmingPass bool    `gorm:"default:false" json:"anti_farming_pass"`

	// Chain link (filled after Galaxy NFT mint in Phase 2).
	NFTTokenID  *uint64 `gorm:"type:bigint;index" json:"nft_token_id,omitempty"`
	NFTTxHash   string  `gorm:"type:varchar(80)" json:"nft_tx_hash,omitempty"`
	NFTOwner    string  `gorm:"type:varchar(64);index" json:"nft_owner,omitempty"`

	// Launched-token link (filled after Phase 3 fair launch).
	TokenAddr   string  `gorm:"type:varchar(64);index" json:"token_addr,omitempty"`

	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// GalaxyRole records a user's role inside one galaxy.
// A single user can hold multiple roles (e.g. founder + contributor).
type GalaxyRole struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GalaxyID  uuid.UUID `gorm:"type:uuid;not null;index:idx_galaxy_role,unique,priority:1" json:"galaxy_id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index:idx_galaxy_role,unique,priority:2" json:"user_id"`
	Role      string    `gorm:"type:varchar(16);not null;index:idx_galaxy_role,unique,priority:3" json:"role"` // founder / contributor / backer / curator
	JoinedAt  time.Time `json:"joined_at"`
	// Reputation within this galaxy (separate from global user reputation).
	Reputation float64  `gorm:"type:decimal(10,4);default:0" json:"reputation"`
}

// Source represents one piece of raw material uploaded by a contributor.
// One Source produces zero or more Atoms after distillation.
type Source struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GalaxyID   uuid.UUID `gorm:"type:uuid;not null;index" json:"galaxy_id"`
	UploaderID uuid.UUID `gorm:"type:uuid;not null;index" json:"uploader_id"`

	Kind       string `gorm:"type:varchar(16);not null" json:"kind"` // markdown / pdf / web / image / text
	URL        string `gorm:"type:text" json:"url,omitempty"`        // remote source (web / arxiv / github)
	FilePath   string `gorm:"type:text" json:"file_path,omitempty"`  // local storage path
	ContentHash string `gorm:"type:varchar(64);uniqueIndex" json:"content_hash"` // sha256, dedupe within galaxy via uniqueIndex(galaxy_id,content_hash) below
	Bytes      int64  `gorm:"default:0" json:"bytes"`
	MimeType   string `gorm:"type:varchar(64)" json:"mime_type,omitempty"`

	// Intake (L1–L2) decision.
	IntakeStatus string `gorm:"type:varchar(20);default:'pending';index" json:"intake_status"` // pending / accepted / rejected
	IntakeReason string `gorm:"type:varchar(64)" json:"intake_reason,omitempty"`               // DUP / OFFTOPIC / SPAM / OK

	// Distillation tracking.
	DistillJobID string `gorm:"type:varchar(64);index" json:"distill_job_id,omitempty"`
	AtomsEmitted int    `gorm:"default:0" json:"atoms_emitted"`

	CreditsCost int `gorm:"default:0" json:"credits_cost"` // deducted at intake-accept time

	CreatedAt time.Time `json:"created_at"`
}

// Atom is one unit of knowledge — a noun-like node OR a verb-like edge,
// distilled by the LLM pipeline from a Source. Each Atom carries its own
// confidence score and is auditable back to its source.
//
// Design note: nodes and edges share one table for simpler Merkle batching
// in Phase 2. Discriminate by Kind ("node" / "edge").
type Atom struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GalaxyID    uuid.UUID `gorm:"type:uuid;not null;index:idx_atom_galaxy_status" json:"galaxy_id"`
	SourceID    uuid.UUID `gorm:"type:uuid;not null;index" json:"source_id"`
	ContribID   uuid.UUID `gorm:"type:uuid;not null;index" json:"contrib_id"` // = user_id of contributor

	Kind        string  `gorm:"type:varchar(8);not null" json:"kind"` // node | edge

	// Node-only fields (Kind = "node").
	NodeLabel   string `gorm:"type:varchar(255)" json:"node_label,omitempty"`
	NodeType    string `gorm:"type:varchar(64);index" json:"node_type,omitempty"` // person / org / concept / event / place / work
	NodeSummary string `gorm:"type:text" json:"node_summary,omitempty"`

	// Edge-only fields (Kind = "edge"). HeadNodeID / TailNodeID point to other Atoms (with Kind="node").
	HeadNodeID  *uuid.UUID `gorm:"type:uuid;index" json:"head_node_id,omitempty"`
	TailNodeID  *uuid.UUID `gorm:"type:uuid;index" json:"tail_node_id,omitempty"`
	EdgeLabel   string     `gorm:"type:varchar(128)" json:"edge_label,omitempty"`
	EdgeDir     string     `gorm:"type:varchar(16);default:'directed'" json:"edge_dir,omitempty"`

	// Quality + alignment.
	Confidence float64 `gorm:"type:decimal(4,3);default:0;index" json:"confidence"`           // 0..1
	AlignedTo  *uuid.UUID `gorm:"type:uuid;index" json:"aligned_to,omitempty"`                // when this atom was merged into another canonical atom
	Ambiguous  bool   `gorm:"default:false;index" json:"ambiguous"`                          // flagged for curator review

	Status     string `gorm:"type:varchar(16);default:'pending';index:idx_atom_galaxy_status" json:"status"` // pending / distilling / accepted / rejected / disputed

	// Chain link (Phase 2).
	MerkleLeaf  string `gorm:"type:varchar(80)" json:"merkle_leaf,omitempty"` // hash that went into the epoch tree
	EpochID     *uuid.UUID `gorm:"type:uuid;index" json:"epoch_id,omitempty"`

	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

// GalaxyApplication tracks the curator-review state of a new galaxy request.
// Distinct from Galaxy so the row history survives if approved/rejected.
type GalaxyApplication struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ApplicantID uuid.UUID `gorm:"type:uuid;not null;index" json:"applicant_id"`
	Slug       string    `gorm:"type:varchar(64);not null;index" json:"slug"`
	Title      string    `gorm:"type:varchar(255);not null" json:"title"`
	Pitch      string    `gorm:"type:text" json:"pitch"`
	Category   string    `gorm:"type:varchar(64)" json:"category"`
	SeedURLs   JSON      `gorm:"type:jsonb;default:'[]'" json:"seed_urls"`

	Status        string  `gorm:"type:varchar(20);default:'pending';index" json:"status"` // pending / approved / rejected
	GalaxyID      *uuid.UUID `gorm:"type:uuid" json:"galaxy_id,omitempty"`                  // filled when approved
	ReviewerID    *uuid.UUID `gorm:"type:uuid" json:"reviewer_id,omitempty"`
	ReviewNote    string  `gorm:"type:text" json:"review_note,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CreditLedger records every Credits debit/credit on the user's V4 wallet.
// Reuses User.Credits as the running balance; this table is the audit log.
//
// Direction: positive Amount = credit (top-up), negative = debit (intake).
type CreditLedger struct {
	ID        uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Amount    int       `gorm:"not null" json:"amount"`                      // signed
	Balance   int       `gorm:"not null" json:"balance"`                     // post-balance snapshot
	Reason    string    `gorm:"type:varchar(32);not null;index" json:"reason"` // topup_crypto / intake / refund / admin_grant
	RefType   string    `gorm:"type:varchar(32)" json:"ref_type,omitempty"`  // source / payment / admin
	RefID     string    `gorm:"type:varchar(64);index" json:"ref_id,omitempty"`
	Note      string    `gorm:"type:text" json:"note,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// Epoch is one Merkle roll-up batch. Per-galaxy when GalaxyID is set;
// nil GalaxyID means a global cross-galaxy epoch (Phase 2.x).
//
// Index is monotonically increasing within (galaxy_id, _) so the UI can show
// "Galaxy X · Epoch #42". Combined with Root (hex sha256), this is the unit
// that gets written to chain in Phase 2.1.
type Epoch struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	GalaxyID   *uuid.UUID `gorm:"type:uuid;index:idx_epoch_galaxy_idx,priority:1" json:"galaxy_id,omitempty"`
	Index      int64      `gorm:"not null;index:idx_epoch_galaxy_idx,priority:2" json:"index"`
	Root       string     `gorm:"type:varchar(80);not null;index" json:"root"` // sha256 hex
	AtomCount  int        `gorm:"not null;default:0" json:"atom_count"`

	// Chain push (Phase 2.1).
	ChainTxHash string     `gorm:"type:varchar(80);index" json:"chain_tx_hash,omitempty"`
	ChainBlock  uint64     `json:"chain_block,omitempty"`
	ChainStatus string     `gorm:"type:varchar(16);default:'pending';index" json:"chain_status"` // pending / confirmed / failed
	PushedAt    *time.Time `json:"pushed_at,omitempty"`

	ClosedAt  time.Time `gorm:"not null" json:"closed_at"`
	CreatedAt time.Time `json:"created_at"`
}
