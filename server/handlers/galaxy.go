// V4 Galaxy handlers — minimum-viable CRUD for Phase 1.
//
// Auth model: REUSES V3 sessions.
//   - Read endpoints (list / get): public.
//   - Write endpoints (apply / approve / upload source): require either
//     an email session OR a wallet session via currentEmailUser /
//     currentWalletUser (see auth_bind.go).
//
// IMPORTANT — DO NOT add a parallel auth scheme here. All V4 roles
// (Founder / Contributor / Backer / Curator) hang off the existing
// models.User.ID via models.GalaxyRole.
package handlers

import (
	"context"
	"errors"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/ensoul-labs/ensoul-server/chain"
	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/services"
	"github.com/ensoul-labs/ensoul-server/services/distill"
	"github.com/ensoul-labs/ensoul-server/services/intake"
	"github.com/ensoul-labs/ensoul-server/services/quality"
	"github.com/ensoul-labs/ensoul-server/util"
	ethcommon "github.com/ethereum/go-ethereum/common"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{2,62}$`)

// resolveV4User returns the active user — email session takes priority
// because that is the V3 default for product UI; falls back to wallet
// session for chain-side flows.
func resolveV4User(c *gin.Context) *models.User {
	if u := currentEmailUser(c); u != nil {
		return u
	}
	if u := currentWalletUser(c); u != nil {
		return u
	}
	return nil
}

// ─── GET /api/v4/galaxy/list?stage=&category=&limit= ─────────────────────────

// GalaxyList returns paginated galaxies. Public.
func GalaxyList(c *gin.Context) {
	stage := strings.TrimSpace(c.Query("stage"))
	category := strings.TrimSpace(c.Query("category"))

	limit := 50
	q := database.DB.Model(&models.Galaxy{}).Order("created_at DESC")
	if stage != "" {
		q = q.Where("stage = ?", stage)
	}
	if category != "" {
		q = q.Where("category = ?", category)
	}

	var rows []models.Galaxy
	if err := q.Limit(limit).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"galaxies": rows})
}

// ─── GET /api/v4/galaxy/:slug ────────────────────────────────────────────────

// GalaxyGet returns a single galaxy by slug. Public.
func GalaxyGet(c *gin.Context) {
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	var g models.Galaxy
	if err := database.DB.Where("slug = ?", slug).First(&g).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, g)
}

// ─── POST /api/v4/galaxy/apply ───────────────────────────────────────────────

type galaxyApplyReq struct {
	Slug     string   `json:"slug"`
	Title    string   `json:"title"`
	Pitch    string   `json:"pitch"`
	Category string   `json:"category"`
	SeedURLs []string `json:"seed_urls"`
}

// GalaxyApply lets a logged-in user submit a galaxy creation request.
// Status starts as "pending"; a curator must approve via GalaxyApprove.
func GalaxyApply(c *gin.Context) {
	user := resolveV4User(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}
	var req galaxyApplyReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Slug = strings.ToLower(strings.TrimSpace(req.Slug))
	req.Title = strings.TrimSpace(req.Title)
	if !slugRe.MatchString(req.Slug) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug must match [a-z0-9][a-z0-9-]{2,62}"})
		return
	}
	if req.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title required"})
		return
	}

	// Reject if slug already taken by an existing (live) galaxy.
	var existing int64
	database.DB.Model(&models.Galaxy{}).Where("slug = ?", req.Slug).Count(&existing)
	if existing > 0 {
		c.JSON(http.StatusConflict, gin.H{"error": "slug already taken"})
		return
	}

	seedJSON := models.JSON{}
	if len(req.SeedURLs) > 0 {
		urls := make([]interface{}, 0, len(req.SeedURLs))
		for _, u := range req.SeedURLs {
			urls = append(urls, u)
		}
		seedJSON = models.JSON{"urls": urls}
	}

	app := models.GalaxyApplication{
		ApplicantID: user.ID,
		Slug:        req.Slug,
		Title:       req.Title,
		Pitch:       req.Pitch,
		Category:    req.Category,
		SeedURLs:    seedJSON,
		Status:      "pending",
	}
	if err := database.DB.Create(&app).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"application": app})
}

// ─── POST /api/v4/galaxy/:id/approve ─────────────────────────────────────────

// GalaxyApprove flips a pending application to approved, creates the Galaxy
// row, mints a founder GalaxyRole, and returns the new Galaxy.
//
// Curator-only in production; for Phase 1.0 local debugging this accepts any
// logged-in user. Replace with `middleware.AuthAdmin()` once admin auth wraps
// V4 endpoints.
func GalaxyApprove(c *gin.Context) {
	user := resolveV4User(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}
	appID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid id"})
		return
	}

	var app models.GalaxyApplication
	if err := database.DB.First(&app, "id = ?", appID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "application not found"})
		return
	}
	if app.Status != "pending" {
		c.JSON(http.StatusConflict, gin.H{"error": "application is " + app.Status})
		return
	}

	var galaxy models.Galaxy
	err = database.DB.Transaction(func(tx *gorm.DB) error {
		galaxy = models.Galaxy{
			Slug:      app.Slug,
			Title:     app.Title,
			Category:  app.Category,
			FounderID: app.ApplicantID,
			Stage:     models.GalaxyStageEmbryo,
		}
		if err := tx.Create(&galaxy).Error; err != nil {
			return err
		}
		if err := tx.Create(&models.GalaxyRole{
			GalaxyID: galaxy.ID, UserID: app.ApplicantID, Role: "founder",
		}).Error; err != nil {
			return err
		}
		app.Status = "approved"
		app.GalaxyID = &galaxy.ID
		app.ReviewerID = &user.ID
		return tx.Save(&app).Error
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Fire-and-forget: mint the Galaxy founder NFT. Failures are logged but
	// don't block approval — the chain row can be back-filled later.
	go mintGalaxyNFTAsync(galaxy, app.ApplicantID)

	c.JSON(http.StatusOK, gin.H{"galaxy": galaxy})
}

// mintGalaxyNFTAsync mints the founder NFT for a newly approved galaxy.
// Looks up the founder's wallet address; skips with a warning if absent or
// if the GalaxyNFT contract isn't configured.
func mintGalaxyNFTAsync(galaxy models.Galaxy, founderID uuid.UUID) {
	var founder models.User
	if err := database.DB.Select("id, wallet_addr").First(&founder, "id = ?", founderID).Error; err != nil {
		util.Log.Warn("[galaxy-nft] founder lookup failed galaxy=%s: %v", galaxy.ID, err)
		return
	}
	if founder.WalletAddr == "" {
		util.Log.Warn("[galaxy-nft] founder %s has no wallet — skip NFT mint for galaxy %s", founderID, galaxy.Slug)
		return
	}
	var gid [32]byte
	b, _ := galaxy.ID.MarshalBinary()
	copy(gid[0:16], b)
	to := ethcommon.HexToAddress(founder.WalletAddr)
	uri := "" // Phase 2.1: metadata URI populated by curator later via setTokenURI
	tx, err := chain.MintGalaxy(context.Background(), to, gid, uri)
	if err != nil {
		if errors.Is(err, chain.ErrGalaxyNFTNotConfigured) {
			util.Log.Warn("[galaxy-nft] skip mint (contract not configured) galaxy=%s", galaxy.Slug)
			return
		}
		util.Log.Error("[galaxy-nft] mint failed galaxy=%s: %v", galaxy.Slug, err)
		return
	}
	// TokenID is parsed asynchronously by a receipt watcher (not in this MVP);
	// for now we record the tx hash + owner. NFTTokenID stays NULL until then.
	if err := database.DB.Model(&models.Galaxy{}).
		Where("id = ?", galaxy.ID).
		Updates(map[string]interface{}{
			"nft_tx_hash": tx,
			"nft_owner":   strings.ToLower(founder.WalletAddr),
		}).Error; err != nil {
		util.Log.Error("[galaxy-nft] persist mint tx failed galaxy=%s: %v", galaxy.Slug, err)
	}
}

// ─── POST /api/v4/galaxy/:slug/source ────────────────────────────────────────

type galaxySourceReq struct {
	Kind string `json:"kind"` // markdown | text | web
	URL  string `json:"url"`
	Text string `json:"text"` // inline text content (for kind=text|markdown)
}

// GalaxySourceUpload registers a new raw source under a galaxy.
//
// Phase 1.0: URL / inline-text only (no multipart file upload yet — that
// arrives in 1.x once we wire object storage). The handler:
//   1) Resolves the contributor (email or wallet session).
//   2) Verifies the galaxy exists and is in a contributable stage.
//   3) Runs intake.Screen() — rejects on DUP / TOO_LARGE / SPAM.
//   4) Deducts Credits via V4LedgerDeduct (writes audit row).
//   5) Persists Source row with intake_status=accepted, queues for distill.
func GalaxySourceUpload(c *gin.Context) {
	user := resolveV4User(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	var galaxy models.Galaxy
	if err := database.DB.Where("slug = ?", slug).First(&galaxy).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
		return
	}
	// Only contributable stages accept sources.
	switch galaxy.Stage {
	case models.GalaxyStageEmbryo, models.GalaxyStageGrowing, models.GalaxyStageMature:
		// ok
	default:
		c.JSON(http.StatusConflict, gin.H{"error": "galaxy not accepting sources in stage " + galaxy.Stage})
		return
	}

	var req galaxySourceReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.Kind = strings.ToLower(strings.TrimSpace(req.Kind))
	req.URL = strings.TrimSpace(req.URL)

	// Determine the bytes that intake will hash + size-check.
	var body []byte
	switch req.Kind {
	case "web":
		if req.URL == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "url required for kind=web"})
			return
		}
		// Phase 1.0: hash the URL itself; fetching happens in distill phase.
		body = []byte(req.URL)
	case "text", "markdown":
		if req.Text == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "text required for kind=text|markdown"})
			return
		}
		body = []byte(req.Text)
	default:
		c.JSON(http.StatusBadRequest, gin.H{"error": "kind must be web|text|markdown"})
		return
	}

	contentHash := intake.HashContent(body)

	// L1 dedupe: build the hash set for this galaxy so intake.Screen can
	// reject in one DB roundtrip.
	var existing []string
	database.DB.Model(&models.Source{}).
		Where("galaxy_id = ?", galaxy.ID).
		Pluck("content_hash", &existing)
	hashSet := make(map[string]bool, len(existing))
	for _, h := range existing {
		hashSet[h] = true
	}

	decision := intake.Screen(c.Request.Context(), &galaxy, body, hashSet)
	if !decision.Accepted {
		c.JSON(http.StatusUnprocessableEntity, gin.H{
			"error":  "rejected by intake",
			"reason": decision.Reason,
		})
		return
	}

	// Credit cost: URL sources are cheaper than full-doc distillation.
	cost := services.CreditCostV4SourceURL
	if req.Kind != "web" {
		cost = services.CreditCostV4SourceDoc
	}

	src := models.Source{
		GalaxyID:     galaxy.ID,
		UploaderID:   user.ID,
		Kind:         req.Kind,
		URL:          req.URL,
		ContentHash:  contentHash,
		Bytes:        int64(len(body)),
		IntakeStatus: "accepted",
		IntakeReason: intake.ReasonOK,
		CreditsCost:  cost,
	}
	if err := database.DB.Create(&src).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Deduct credits AFTER the row exists so the ledger ref_id is valid.
	if _, err := services.V4LedgerDeduct(user.ID, cost, "v4_source_intake", "source", src.ID.String()); err != nil {
		// Roll back the source row so the user isn't stuck with an
		// orphaned, unpaid intake. Credit deduction failed (likely
		// insufficient balance), so it's safe to delete.
		database.DB.Delete(&models.Source{}, "id = ?", src.ID)
		c.JSON(http.StatusPaymentRequired, gin.H{"error": err.Error()})
		return
	}

	// Ensure contributor has a GalaxyRole row.
	database.DB.FirstOrCreate(&models.GalaxyRole{}, models.GalaxyRole{
		GalaxyID: galaxy.ID, UserID: user.ID, Role: "contributor",
	})

	// Kick off distillation asynchronously for text/markdown sources.
	// Web sources need a fetch step (not in Phase 1.0) before distill.
	if req.Kind == "text" || req.Kind == "markdown" {
		go runDistillAsync(galaxy.ID, src.ID, user.ID, req.Text)
	}

	c.JSON(http.StatusOK, gin.H{"source": src})
}

// runDistillAsync runs distill.Run in the background. Failures are logged
// and the Source row's intake_status is left untouched (atoms_emitted stays
// 0). A future Phase 1.x reconciliation job will retry.
func runDistillAsync(galaxyID, sourceID, contribID uuid.UUID, text string) {
	defer func() {
		if r := recover(); r != nil {
			util.Log.Error("distill panic galaxy=%s source=%s: %v", galaxyID, sourceID, r)
		}
	}()
	// Cap any one distillation at 90s — the LLM call dominates.
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	res, err := distill.Run(ctx, database.DB, distill.Job{
		GalaxyID:  galaxyID,
		SourceID:  sourceID,
		ContribID: contribID,
		Text:      text,
	})
	if err != nil {
		util.Log.Error("distill failed galaxy=%s source=%s: %v", galaxyID, sourceID, err)
		return
	}
	emitted := len(res.Nodes) + len(res.Edges)
	database.DB.Model(&models.Source{}).
		Where("id = ?", sourceID).
		Update("atoms_emitted", emitted)
	// Recompute galaxy-level quality snapshot. Cheap (a few aggregate
	// queries); fine to run inline in this goroutine.
	if _, err := quality.Recompute(galaxyID); err != nil {
		util.Log.Warn("quality recompute failed galaxy=%s: %v", galaxyID, err)
	}
	util.Log.Info("distill ok galaxy=%s source=%s atoms=%d", galaxyID, sourceID, emitted)
}

// ─── GET /api/v4/galaxy/:slug/atoms ──────────────────────────────────────────

// GalaxyAtoms returns paginated atoms (nodes + edges) of a galaxy.
// Public read endpoint. Query params:
//   - kind:    node | edge | "" (both)
//   - status:  default = "accepted"
//   - limit:   default 100, max 500
func GalaxyAtoms(c *gin.Context) {
	slug := strings.ToLower(strings.TrimSpace(c.Param("slug")))
	var galaxy models.Galaxy
	if err := database.DB.Select("id").Where("slug = ?", slug).First(&galaxy).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
		return
	}

	status := strings.TrimSpace(c.DefaultQuery("status", models.AtomStatusAccepted))
	kind := strings.TrimSpace(c.Query("kind"))

	q := database.DB.Model(&models.Atom{}).
		Where("galaxy_id = ? AND status = ?", galaxy.ID, status).
		Order("created_at DESC")
	if kind == "node" || kind == "edge" {
		q = q.Where("kind = ?", kind)
	}

	var rows []models.Atom
	if err := q.Limit(200).Find(&rows).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"atoms": rows})
}
