// V3 → V4 数据迁移命令（一次性脚本）。
//
// 用法：
//   go run ./cmd/migrate_v3_to_v4 -dry=true              # 只打印计划
//   go run ./cmd/migrate_v3_to_v4 -dry=false             # 真跑
//   go run ./cmd/migrate_v3_to_v4 -only-shell=<handle>   # 只迁一个 Soul
//
// 策略：
//   1. 每个 V3 Shell (Soul) → V4 Galaxy
//      - slug = handle (lowercase)
//      - title = display_name
//      - category = "soul"
//      - founder_id：先按 Shell.OwnerAddr 查 users.wallet_addr；查不到用 fallback admin
//   2. 每个 V3 Fragment → V4 Atom (kind=node)
//      - node_label = Content 前 80 字符
//      - 每个 Shell 创建一条 Source(kind=web) 承载所有 Fragment
//      - status: V3 status="accepted" → AtomStatusAccepted；其余 → AtomStatusPending
//
// Idempotent：靠 Galaxy.slug 唯一约束 + Source.content_hash 派生稳定值实现。
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/google/uuid"

	"github.com/ensoul-labs/ensoul-server/config"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
)

func main() {
	var (
		dry       = flag.Bool("dry", true, "dry-run; nothing is written")
		onlyShell = flag.String("only-shell", "", "migrate a single Shell by handle")
		minFrag   = flag.Int("min-fragments", 1, "skip Shells with fewer than N Fragments")
		maxShells = flag.Int("max-shells", 0, "0 = no cap; otherwise limit how many Shells this run")
	)
	flag.Parse()

	cfg := config.Load()
	database.Connect(cfg)

	m := &migrator{dry: *dry}
	if err := m.run(*onlyShell, *minFrag, *maxShells); err != nil {
		fmt.Fprintln(os.Stderr, "ERROR:", err)
		os.Exit(1)
	}
	fmt.Printf("\n✓ done — galaxies=%d  atoms=%d  shells_skipped=%d  frags_skipped=%d  (dry=%v)\n",
		m.galaxiesCreated, m.atomsCreated, m.shellsSkipped, m.fragsSkipped, m.dry)
}

type migrator struct {
	dry             bool
	galaxiesCreated int
	atomsCreated    int
	shellsSkipped   int
	fragsSkipped    int
	fallbackAdminID uuid.UUID
}

func (m *migrator) run(onlyShell string, minFrag, maxShells int) error {
	// Fallback founder：取最老的 user（V3 admin 在 users 表里通常排第一）。
	var u models.User
	database.DB.Order("created_at ASC").First(&u)
	m.fallbackAdminID = u.ID
	if m.fallbackAdminID == uuid.Nil {
		return fmt.Errorf("no user found to use as founder fallback")
	}
	fmt.Printf("fallback founder user_id = %s\n", m.fallbackAdminID)

	q := database.DB.Model(&models.Shell{}).Order("handle ASC")
	if onlyShell != "" {
		q = q.Where("LOWER(handle) = ?", strings.ToLower(onlyShell))
	}
	if maxShells > 0 {
		q = q.Limit(maxShells)
	}
	var shells []models.Shell
	if err := q.Find(&shells).Error; err != nil {
		return fmt.Errorf("load shells: %w", err)
	}
	fmt.Printf("considering %d shells\n", len(shells))

	for _, s := range shells {
		if err := m.migrateOne(s, minFrag); err != nil {
			fmt.Printf("  ! shell %s failed: %v\n", s.Handle, err)
			m.shellsSkipped++
		}
	}
	return nil
}

func (m *migrator) migrateOne(s models.Shell, minFrag int) error {
	slug := strings.ToLower(strings.TrimSpace(s.Handle))
	if slug == "" {
		m.shellsSkipped++
		return fmt.Errorf("empty handle")
	}

	var existing models.Galaxy
	if err := database.DB.Where("slug = ?", slug).First(&existing).Error; err == nil {
		fmt.Printf("  · %s already migrated (galaxy=%s)\n", slug, existing.ID)
		m.shellsSkipped++
		return nil
	}

	founderID := m.fallbackAdminID
	if s.OwnerAddr != "" {
		var u models.User
		if err := database.DB.Where("LOWER(wallet_addr) = ?", strings.ToLower(s.OwnerAddr)).First(&u).Error; err == nil {
			founderID = u.ID
		}
	}

	var frags []models.Fragment
	if err := database.DB.Where("shell_id = ?", s.ID).Order("created_at ASC").Find(&frags).Error; err != nil {
		return fmt.Errorf("load fragments: %w", err)
	}
	if len(frags) < minFrag {
		fmt.Printf("  · %s only %d fragments (< min=%d)\n", slug, len(frags), minFrag)
		m.shellsSkipped++
		return nil
	}

	fmt.Printf("  → %s: %d fragments → 1 galaxy + atoms\n", slug, len(frags))
	if m.dry {
		return nil
	}

	g := models.Galaxy{
		Slug:      slug,
		Title:     firstNonEmpty(s.DisplayName, s.Handle),
		Subtitle:  "Migrated from V3 Soul @" + s.Handle,
		Category:  "soul",
		Lang:      "en",
		FounderID: founderID,
		Stage:     models.GalaxyStageGrowing,
		AtomCount: len(frags),
		NodeCount: len(frags),
		CreatedAt: s.CreatedAt,
	}
	if err := database.DB.Create(&g).Error; err != nil {
		return fmt.Errorf("create galaxy: %w", err)
	}
	m.galaxiesCreated++

	hashBytes := sha256.Sum256([]byte("v3:soul:" + s.Handle))
	src := models.Source{
		GalaxyID:     g.ID,
		UploaderID:   founderID,
		Kind:         "web",
		URL:          "https://twitter.com/" + s.Handle,
		ContentHash:  hex.EncodeToString(hashBytes[:]),
		MimeType:     "text/plain",
		IntakeStatus: "accepted",
		CreatedAt:    s.CreatedAt,
	}
	if err := database.DB.Create(&src).Error; err != nil {
		return fmt.Errorf("create source: %w", err)
	}

	for _, f := range frags {
		label := truncate(strings.TrimSpace(f.Content), 80)
		if label == "" {
			m.fragsSkipped++
			continue
		}
		conf := f.Confidence
		if conf <= 0 {
			conf = 0.5
		}
		status := models.AtomStatusPending
		if strings.EqualFold(f.Status, "accepted") {
			status = models.AtomStatusAccepted
		}
		a := models.Atom{
			GalaxyID:    g.ID,
			SourceID:    src.ID,
			ContribID:   founderID,
			Kind:        "node",
			NodeLabel:   label,
			NodeType:    "tweet",
			NodeSummary: f.Content,
			Confidence:  conf,
			Status:      status,
			CreatedAt:   f.CreatedAt,
		}
		if err := database.DB.Create(&a).Error; err != nil {
			fmt.Printf("    ! fragment %s skipped: %v\n", f.ID, err)
			m.fragsSkipped++
			continue
		}
		m.atomsCreated++
	}
	return nil
}

func firstNonEmpty(ss ...string) string {
	for _, s := range ss {
		if strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}
