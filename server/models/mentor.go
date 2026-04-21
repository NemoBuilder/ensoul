package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Mentor methodology categories.
const (
	MentorCategoryReference   = "reference"    // 长篇操作手册（writing/algorithm/growth/quality）
	MentorCategoryMentalModel = "mental_model" // 6 个核心心智模型
	MentorCategoryHeuristic   = "heuristic"    // 10 条决策启发式
	MentorCategoryRouting     = "routing"      // 场景路由表（SKILL.md 主表）
)

// MentorMethodology stores a single methodology record (reference chapter / mental model / heuristic / routing).
//
// Records are seeded from `mydocs/methodology/x-mentor-v2.0/` (MIT, by 花叔 @AlchainHust).
// Each record is uniquely identified by (slug, source, locale).
//
// Loading strategy at runtime (handled by handler/service layer):
//   - Always inject: routing + all heuristics (compact)
//   - Scenario-conditional: load relevant references / mental_models on demand
type MentorMethodology struct {
	ID uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`

	// Classification
	Category string `gorm:"type:varchar(32);not null;index:idx_mentor_cat_locale" json:"category"`
	Slug     string `gorm:"type:varchar(120);not null;uniqueIndex:idx_mentor_slug_src_loc,priority:1" json:"slug"`
	Locale   string `gorm:"type:varchar(8);not null;default:'zh';index:idx_mentor_cat_locale;uniqueIndex:idx_mentor_slug_src_loc,priority:3" json:"locale"`

	// Content
	Title   string `gorm:"type:varchar(255);not null" json:"title"`
	Summary string `gorm:"type:text" json:"summary"`
	BodyMD  string `gorm:"type:text;not null" json:"body_md"`
	Tags    string `gorm:"type:text" json:"tags"` // comma-separated; e.g. "hook,thread,scene_a"

	// Source attribution (critical for upgrade safety)
	Source    string `gorm:"type:varchar(64);not null;default:'internal-ensoul';uniqueIndex:idx_mentor_slug_src_loc,priority:2" json:"source"`
	SourceURL string `gorm:"type:varchar(500)" json:"source_url"`
	Version   string `gorm:"type:varchar(32);default:'1.0'" json:"version"`

	// Loading control
	Enabled  bool `gorm:"not null;default:true;index" json:"enabled"`
	Priority int  `gorm:"not null;default:50" json:"priority"` // 0-100, higher = preferred when context-budget tight

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// TableName overrides the table name.
func (MentorMethodology) TableName() string {
	return "mentor_methodologies"
}
