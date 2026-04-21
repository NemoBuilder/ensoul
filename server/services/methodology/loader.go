package methodology

import (
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/ensoul-labs/ensoul-server/models"
	"gorm.io/gorm"
)

// Scenario identifies the user's intent for methodology routing.
//
// Maps to SKILL.md routing table:
//   - A: write tweet/thread
//   - B: brainstorm topic / no inspiration
//   - C: review existing content
//   - D: growth/strategy
//   - E: account diagnosis
//   - General: default fallback (heuristics only)
type Scenario string

const (
	ScenarioWriting   Scenario = "A_writing"
	ScenarioTopic     Scenario = "B_topic"
	ScenarioReview    Scenario = "C_review"
	ScenarioGrowth    Scenario = "D_growth"
	ScenarioDiagnosis Scenario = "E_diagnosis"
	ScenarioMemory    Scenario = "F_memory"
	ScenarioGeneral   Scenario = "general"
)

// scenarioRefs maps each scenario to the reference slugs to load (full body).
var scenarioRefs = map[Scenario][]string{
	ScenarioWriting:   {"ref-writing-workshop", "ref-algorithm-niche"},
	ScenarioTopic:     {"ref-writing-workshop"},
	ScenarioReview:    {"ref-quality-analytics", "ref-writing-workshop"},
	ScenarioGrowth:    {"ref-growth-monetization", "ref-algorithm-niche"},
	ScenarioDiagnosis: {"ref-quality-analytics"},
	ScenarioMemory:    {}, // memory updates: skip methodology, let LLM focus on memory-suggest
	ScenarioGeneral:   {},
}

// scenarioLoadsMentalModels: scenarios B (topic brainstorm) loads mental_models summaries.
var scenarioLoadsMentalModels = map[Scenario]bool{
	ScenarioTopic: true,
}

// DetectScenario uses lightweight keyword rules to classify the user message.
// Returns ScenarioGeneral when no rule matches; LLM fallback can be added later.
func DetectScenario(userMessage string) Scenario {
	msg := strings.ToLower(strings.TrimSpace(userMessage))
	if msg == "" {
		return ScenarioGeneral
	}

	// Order matters: more specific patterns first.
	switch {
	case matchAny(msg, reMemory):
		return ScenarioMemory
	case matchAny(msg, reReview):
		return ScenarioReview
	case matchAny(msg, reDiagnosis):
		return ScenarioDiagnosis
	case matchAny(msg, reTopic):
		return ScenarioTopic
	case matchAny(msg, reGrowth):
		return ScenarioGrowth
	case matchAny(msg, reWriting):
		return ScenarioWriting
	}
	return ScenarioGeneral
}

var (
	// 审阅：明确包含 已写/帮我看看/审阅/check/review/打分
	reReview = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(帮我看|帮看|审阅|审一下|点评|打分|review|critique|这条.{0,8}(怎么样|如何|可以吗))`),
	}
	// 诊断：账号/数据分析
	reDiagnosis = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(诊断|分析.*账号|账号.*分析|数据复盘|分析报告|audit|analyz.*account)`),
	}
	// 选题：没灵感/选题/想不出来/写什么/topic/brainstorm
	reTopic = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(选题|没灵感|没思路|想不出|不知道(写|发)什么|发什么好|topic|brainstorm|tweet idea|ideas? for.*tweet|what.*post|what should i (tweet|post|write))`),
	}
	// 增长：粉丝/涨粉/增长/策略/growth/follower/strategy
	reGrowth = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(涨粉|粉丝|增长|破圈|做大|策略.*x|x.*策略|growth|follower|grow.*x|scale.*account)`),
	}
	// 写推文：写/帮我写/起草/草稿/tweet/thread/draft/write
	reWriting = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(写一条|写一段|写个|写条|帮我写|起草|草稿|改写|润色|翻译.*推|发条|发个|draft|write.*tweet|write.*thread|rewrite|polish)`),
	}
	// 记忆更新：更新定位/修改资料/记住/save to profile/remember
	reMemory = []*regexp.Regexp{
		regexp.MustCompile(`(?i)(更新|修改|改一下|调整).{0,4}(定位|资料|档案|profile|介绍|简介|规则|rules|记忆|memory|knowledge|网络|network)`),
		regexp.MustCompile(`(?i)(记住|帮我记|保存(到|进).*(记忆|档案|profile|memory)|存(到|进).*(记忆|档案|profile|memory))`),
		regexp.MustCompile(`(?i)(remember (this|that)|save.{0,8}(to|in|into).{0,8}(profile|memory|knowledge|rules|network|archive)|update (my )?(profile|rules|memory|knowledge))`),
	}
)

func matchAny(s string, patterns []*regexp.Regexp) bool {
	for _, p := range patterns {
		if p.MatchString(s) {
			return true
		}
	}
	return false
}

// LoadOptions controls how much methodology to inject.
type LoadOptions struct {
	Source string // attribution filter, default "x-mentor-skill@v2.0"
	Locale string // default "zh"
	// MaxBodyChars caps the total characters of full-body reference content
	// injected. 0 means no cap. Each reference body is ~3-5K chars.
	MaxBodyChars int
}

// LoadResult is the structured methodology bundle returned by Load.
type LoadResult struct {
	Scenario     Scenario
	Routing      *models.MentorMethodology   // SKILL routing record (always present if found)
	Heuristics   []models.MentorMethodology  // 10 decision heuristics (compact, always)
	References   []models.MentorMethodology  // scenario-conditional, full body
	MentalModels []models.MentorMethodology  // scenario-conditional, summaries only
	UsedSlugs    []string                    // for telemetry
}

// Load fetches the methodology bundle for a given user message.
func Load(db *gorm.DB, userMessage string, opt LoadOptions) (*LoadResult, error) {
	if opt.Source == "" {
		opt.Source = "x-mentor-skill@v2.0"
	}
	if opt.Locale == "" {
		opt.Locale = "zh"
	}
	scenario := DetectScenario(userMessage)
	res := &LoadResult{Scenario: scenario}

	base := db.Model(&models.MentorMethodology{}).
		Where("source = ? AND locale = ? AND enabled = ?", opt.Source, opt.Locale, true)

	// 1. Routing (always)
	var routing models.MentorMethodology
	if err := base.Session(&gorm.Session{}).
		Where("category = ?", models.MentorCategoryRouting).
		Order("priority DESC").First(&routing).Error; err == nil {
		res.Routing = &routing
		res.UsedSlugs = append(res.UsedSlugs, routing.Slug)
	}

	// 2. Heuristics (always, all 10)
	if err := base.Session(&gorm.Session{}).
		Where("category = ?", models.MentorCategoryHeuristic).
		Order("priority DESC, slug ASC").
		Find(&res.Heuristics).Error; err != nil {
		return nil, fmt.Errorf("load heuristics: %w", err)
	}
	for _, h := range res.Heuristics {
		res.UsedSlugs = append(res.UsedSlugs, h.Slug)
	}

	// 3. References (scenario-conditional)
	refSlugs := scenarioRefs[scenario]
	if len(refSlugs) > 0 {
		var refs []models.MentorMethodology
		if err := base.Session(&gorm.Session{}).
			Where("category = ? AND slug IN ?", models.MentorCategoryReference, refSlugs).
			Find(&refs).Error; err != nil {
			return nil, fmt.Errorf("load refs: %w", err)
		}
		// preserve scenarioRefs order for stable output
		bySlug := map[string]models.MentorMethodology{}
		for _, r := range refs {
			bySlug[r.Slug] = r
		}
		used := 0
		for _, slug := range refSlugs {
			r, ok := bySlug[slug]
			if !ok {
				continue
			}
			if opt.MaxBodyChars > 0 && used+len(r.BodyMD) > opt.MaxBodyChars {
				// truncate this reference rather than skip entirely
				remain := opt.MaxBodyChars - used
				if remain > 500 { // only worth including if meaningful chunk fits
					r.BodyMD = r.BodyMD[:remain] + "\n\n[...truncated for context budget]"
					res.References = append(res.References, r)
					res.UsedSlugs = append(res.UsedSlugs, r.Slug+"#truncated")
				}
				break
			}
			res.References = append(res.References, r)
			res.UsedSlugs = append(res.UsedSlugs, r.Slug)
			used += len(r.BodyMD)
		}
	}

	// 4. Mental models (scenario-conditional, summaries only)
	if scenarioLoadsMentalModels[scenario] {
		if err := base.Session(&gorm.Session{}).
			Where("category = ?", models.MentorCategoryMentalModel).
			Order("slug ASC").
			Find(&res.MentalModels).Error; err != nil {
			return nil, fmt.Errorf("load mental models: %w", err)
		}
		for _, m := range res.MentalModels {
			res.UsedSlugs = append(res.UsedSlugs, m.Slug)
		}
	}

	sort.Strings(res.UsedSlugs)
	return res, nil
}

// RenderPromptSection produces a markdown block ready to be appended to the
// LLM system prompt. Returns empty string if no methodology was loaded.
func (r *LoadResult) RenderPromptSection() string {
	if r == nil || (len(r.Heuristics) == 0 && r.Routing == nil) {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("\n\n## 导师方法论（来自 x-mentor-skill@v2.0，MIT，by 花叔 @AlchainHust）\n")
	sb.WriteString("> 自动应用以下方法论；除非用户明确询问，否则不要直接引用方法论名称，而是把建议自然融入回答。\n\n")

	// Heuristics: compact title + summary
	if len(r.Heuristics) > 0 {
		sb.WriteString("### 决策启发式（始终遵循）\n")
		for _, h := range r.Heuristics {
			sb.WriteString("- **")
			sb.WriteString(strings.TrimSpace(h.Title))
			sb.WriteString("**")
			if h.Summary != "" {
				sb.WriteString("：")
				sb.WriteString(strings.TrimSpace(h.Summary))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// Mental models: summaries only
	if len(r.MentalModels) > 0 {
		sb.WriteString("### 核心心智模型（参考）\n")
		for _, m := range r.MentalModels {
			sb.WriteString("- **")
			sb.WriteString(strings.TrimSpace(m.Title))
			sb.WriteString("**")
			if m.Summary != "" {
				sb.WriteString("：")
				sb.WriteString(strings.TrimSpace(m.Summary))
			}
			sb.WriteString("\n")
		}
		sb.WriteString("\n")
	}

	// References: full body
	for _, ref := range r.References {
		sb.WriteString("### 参考手册：")
		sb.WriteString(strings.TrimSpace(ref.Title))
		sb.WriteString("\n\n")
		sb.WriteString(strings.TrimSpace(ref.BodyMD))
		sb.WriteString("\n\n")
	}

	return sb.String()
}
