// Package methodology provides parsing and seeding of mentor methodology
// markdown packs into the mentor_methodologies table.
//
// A "pack" is a directory like data/methodology/x-mentor-v2.0/ containing:
//   - SKILL.md             → 1 routing record
//   - references/*.md      → 4 reference records + 1 mental-models-heuristics combo
//   - LICENSE, CREDITS.md  → metadata only (not seeded)
//
// Source-attribution rule:
//   - Records carry source = pack tag (e.g. "x-mentor-skill@v2.0")
//   - Records with source = "internal-ensoul" are NEVER touched
//   - Existing same-source records are only overwritten when force=true
package methodology

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/ensoul-labs/ensoul-server/models"
	"gorm.io/gorm"
)

// PackSpec describes one methodology pack to seed.
type PackSpec struct {
	Dir       string // absolute or cwd-relative path to pack directory
	Source    string // attribution tag, e.g. "x-mentor-skill@v2.0"
	SourceURL string
	Locale    string // e.g. "zh"
	Version   string // e.g. "2.0"
}

// SeedResult reports outcome of one seed run.
type SeedResult struct {
	Inserted int
	Updated  int
	Skipped  bool   // true when records already present and force=false
	Reason   string // human-readable note
}

// SeedPack parses the pack at spec.Dir and writes records.
//   - force=false: if any same-source records already exist, skip (idempotent on restart)
//   - force=true:  upsert every record
func SeedPack(db *gorm.DB, spec PackSpec, force bool) (*SeedResult, error) {
	if spec.Locale == "" {
		spec.Locale = "zh"
	}
	if _, err := os.Stat(spec.Dir); err != nil {
		return nil, fmt.Errorf("pack dir not accessible: %w", err)
	}

	records, err := parsePack(spec.Dir)
	if err != nil {
		return nil, fmt.Errorf("parse: %w", err)
	}

	var existing int64
	db.Model(&models.MentorMethodology{}).
		Where("source = ? AND locale = ?", spec.Source, spec.Locale).
		Count(&existing)
	if existing > 0 && !force {
		return &SeedResult{Skipped: true, Reason: fmt.Sprintf("%d existing records for source=%s locale=%s; use force=true to overwrite", existing, spec.Source, spec.Locale)}, nil
	}

	res := &SeedResult{}
	for _, r := range records {
		var existing models.MentorMethodology
		err := db.Where("slug = ? AND source = ? AND locale = ?", r.Slug, spec.Source, spec.Locale).
			First(&existing).Error
		if err == gorm.ErrRecordNotFound {
			row := models.MentorMethodology{
				Category:  r.Category,
				Slug:      r.Slug,
				Locale:    spec.Locale,
				Title:     r.Title,
				Summary:   r.Summary,
				BodyMD:    r.BodyMD,
				Tags:      r.Tags,
				Source:    spec.Source,
				SourceURL: spec.SourceURL,
				Version:   spec.Version,
				Enabled:   true,
				Priority:  r.Priority,
			}
			if err := db.Create(&row).Error; err != nil {
				return res, fmt.Errorf("create %s: %w", r.Slug, err)
			}
			res.Inserted++
		} else if err != nil {
			return res, fmt.Errorf("query %s: %w", r.Slug, err)
		} else {
			existing.Category = r.Category
			existing.Title = r.Title
			existing.Summary = r.Summary
			existing.BodyMD = r.BodyMD
			existing.Tags = r.Tags
			existing.Version = spec.Version
			existing.SourceURL = spec.SourceURL
			existing.Priority = r.Priority
			existing.Enabled = true
			if err := db.Save(&existing).Error; err != nil {
				return res, fmt.Errorf("update %s: %w", r.Slug, err)
			}
			res.Updated++
		}
	}
	return res, nil
}

// CountBySource returns number of records currently in DB for given source+locale.
func CountBySource(db *gorm.DB, source, locale string) int64 {
	var n int64
	db.Model(&models.MentorMethodology{}).
		Where("source = ? AND locale = ?", source, locale).
		Count(&n)
	return n
}

// ─── Parsing ─────────────────────────────────────────────────────────────────

type parsedRecord struct {
	Slug     string
	Category string
	Title    string
	Summary  string
	BodyMD   string
	Tags     string
	Priority int
}

func parsePack(dir string) ([]parsedRecord, error) {
	var recs []parsedRecord

	refSpecs := []struct {
		File     string
		Slug     string
		Title    string
		Tags     string
		Priority int
	}{
		{"references/writing-workshop.md", "ref-writing-workshop", "写作工坊（短推文/Hook/Thread/选题）", "scene_a,scene_b,scene_c,hook,thread,topic,writing", 80},
		{"references/algorithm-niche.md", "ref-algorithm-niche", "X 算法速查 + AI 赛道专精", "algorithm,ai_niche,timing,scene_a,scene_d", 75},
		{"references/growth-monetization.md", "ref-growth-monetization", "增长引擎 + 变现路径 + 流派对比", "growth,monetization,scene_d", 70},
		{"references/quality-analytics.md", "ref-quality-analytics", "质量检查 + 反模式 + 数据复盘 + 报告模板", "quality,analytics,diagnosis,scene_c,scene_e", 75},
	}
	for _, spec := range refSpecs {
		body, err := os.ReadFile(filepath.Join(dir, spec.File))
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", spec.File, err)
		}
		recs = append(recs, parsedRecord{
			Slug:     spec.Slug,
			Category: models.MentorCategoryReference,
			Title:    spec.Title,
			Summary:  extractSummary(string(body)),
			BodyMD:   string(body),
			Tags:     spec.Tags,
			Priority: spec.Priority,
		})
	}

	mmBytes, err := os.ReadFile(filepath.Join(dir, "references/mental-models-heuristics.md"))
	if err != nil {
		return nil, fmt.Errorf("read mental-models: %w", err)
	}
	mm, hr, err := splitMentalModels(string(mmBytes))
	if err != nil {
		return nil, err
	}
	recs = append(recs, mm...)
	recs = append(recs, hr...)

	skillBytes, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil, fmt.Errorf("read SKILL.md: %w", err)
	}
	recs = append(recs, parsedRecord{
		Slug:     "routing-main",
		Category: models.MentorCategoryRouting,
		Title:    "X 导师场景路由表（主入口）",
		Summary:  "根据用户问题类型路由到 5 个执行场景（A 写推文 / B 选题 / C 审阅 / D 增长 / E 诊断），并指引按需加载对应 reference。",
		BodyMD:   string(skillBytes),
		Tags:     "routing,scene_router,always_load",
		Priority: 100,
	})

	return recs, nil
}

func extractSummary(body string) string {
	for _, l := range strings.Split(body, "\n") {
		t := strings.TrimSpace(l)
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, ">") || strings.HasPrefix(t, "---") {
			continue
		}
		if len(t) > 200 {
			t = t[:200] + "..."
		}
		return t
	}
	return ""
}

var (
	reModelHeading     = regexp.MustCompile(`^### 模型(\d+):\s*(.+)$`)
	reHeuristicHeading = regexp.MustCompile(`^### (\d+)\.\s*(.+?)(?:\s*←.*)?$`)
	reHeuristicSection = regexp.MustCompile(`^##\s+决策启发式`)
	reModelSection     = regexp.MustCompile(`^##\s+核心心智模型`)
)

func splitMentalModels(body string) (models_ []parsedRecord, heuristics []parsedRecord, err error) {
	type chunk struct {
		Heading string
		Idx     int
		Content []string
	}
	var (
		currentSection string
		current        *chunk
		modelChunks    []chunk
		heurChunks     []chunk
	)
	flush := func() {
		if current == nil {
			return
		}
		switch currentSection {
		case "model":
			modelChunks = append(modelChunks, *current)
		case "heuristic":
			heurChunks = append(heurChunks, *current)
		}
		current = nil
	}
	for _, line := range strings.Split(body, "\n") {
		if reModelSection.MatchString(line) {
			flush()
			currentSection = "model"
			continue
		}
		if reHeuristicSection.MatchString(line) {
			flush()
			currentSection = "heuristic"
			continue
		}
		if currentSection == "model" {
			if m := reModelHeading.FindStringSubmatch(line); m != nil {
				flush()
				current = &chunk{Heading: strings.TrimSpace(m[2]), Idx: atoiSafe(m[1])}
				continue
			}
		}
		if currentSection == "heuristic" {
			if m := reHeuristicHeading.FindStringSubmatch(line); m != nil {
				flush()
				current = &chunk{Heading: strings.TrimSpace(m[2]), Idx: atoiSafe(m[1])}
				continue
			}
		}
		if current != nil {
			current.Content = append(current.Content, line)
		}
	}
	flush()

	if len(modelChunks) != 6 {
		return nil, nil, fmt.Errorf("expected 6 mental models, got %d", len(modelChunks))
	}
	if len(heurChunks) != 10 {
		return nil, nil, fmt.Errorf("expected 10 heuristics, got %d", len(heurChunks))
	}

	for _, c := range modelChunks {
		body := strings.TrimSpace(strings.Join(c.Content, "\n"))
		models_ = append(models_, parsedRecord{
			Slug:     fmt.Sprintf("mental-model-%02d-%s", c.Idx, slugify(c.Heading)),
			Category: models.MentorCategoryMentalModel,
			Title:    fmt.Sprintf("心智模型%d：%s", c.Idx, c.Heading),
			Summary:  firstQuotedSentence(body),
			BodyMD:   "### " + fmt.Sprintf("模型%d: %s", c.Idx, c.Heading) + "\n\n" + body,
			Tags:     "mental_model,framework",
			Priority: 60,
		})
	}
	for _, c := range heurChunks {
		body := strings.TrimSpace(strings.Join(c.Content, "\n"))
		heuristics = append(heuristics, parsedRecord{
			Slug:     fmt.Sprintf("heuristic-%02d-%s", c.Idx, slugify(c.Heading)),
			Category: models.MentorCategoryHeuristic,
			Title:    fmt.Sprintf("启发式%d：%s", c.Idx, c.Heading),
			Summary:  firstNonEmptyLine(body),
			BodyMD:   "### " + fmt.Sprintf("%d. %s", c.Idx, c.Heading) + "\n\n" + body,
			Tags:     "heuristic,decision_rule,always_load",
			Priority: 90,
		})
	}
	return models_, heuristics, nil
}

func firstQuotedSentence(s string) string {
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "**一句话**") {
			parts := strings.SplitN(t, "：", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return firstNonEmptyLine(s)
}

func firstNonEmptyLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		t := strings.TrimSpace(line)
		if t == "" || strings.HasPrefix(t, "#") {
			continue
		}
		if len(t) > 200 {
			t = t[:200] + "..."
		}
		return t
	}
	return ""
}

var reSlugStrip = regexp.MustCompile(`[^a-z0-9\-]+`)

func slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var asciiBuf strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			asciiBuf.WriteRune(r)
		} else {
			asciiBuf.WriteRune('-')
		}
	}
	out := reSlugStrip.ReplaceAllString(asciiBuf.String(), "-")
	out = strings.Trim(out, "-")
	if out == "" {
		var firstCP rune
		for _, r := range s {
			firstCP = r
			break
		}
		out = fmt.Sprintf("cjk-%d-%x", len([]rune(s)), firstCP)
	}
	if len(out) > 60 {
		out = out[:60]
	}
	return out
}

func atoiSafe(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return n
		}
		n = n*10 + int(c-'0')
	}
	return n
}

// ─── Default pack ────────────────────────────────────────────────────────────

// DefaultPack returns the spec for the bundled x-mentor-skill@v2.0 pack.
// Allows METHODOLOGY_DIR env override for non-standard deploy layouts.
func DefaultPack() PackSpec {
	dir := os.Getenv("METHODOLOGY_DIR")
	if dir == "" {
		dir = "data/methodology/x-mentor-v2.0"
	}
	return PackSpec{
		Dir:       dir,
		Source:    "x-mentor-skill@v2.0",
		SourceURL: "https://github.com/alchaincyf/x-mentor-skill",
		Locale:    "zh",
		Version:   "2.0",
	}
}
