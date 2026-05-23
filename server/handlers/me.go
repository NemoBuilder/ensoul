// Package handlers — V4 「我的贡献」聚合接口。
//
// GET /api/v4/me/contributions
//   - 按 galaxy 聚合当前用户贡献的 atoms 数量（按状态拆分）
//   - 返回 per-galaxy 贡献度 + 全局汇总
//
// 设计选择：不引入独立的 points 表 — 直接用 Atom.ContribID + Galaxy 名称
// 实时 GROUP BY 算出，避免又一张冗余表。每个用户的 atom 数量在 V4 早期都不会
// 大到无法聚合，DB 扛得住。后续如需细化（reputation 加权 / 时间窗口），
// 再加 materialized view 或 services/mining_points.go 真正实现。
package handlers

import (
	"net/http"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/gin-gonic/gin"
)

type contribRow struct {
	GalaxyID      string `json:"galaxy_id"`
	GalaxySlug    string `json:"galaxy_slug"`
	GalaxyTitle   string `json:"galaxy_title"`
	Accepted      int64  `json:"accepted"`
	Pending       int64  `json:"pending"`
	Disputed      int64  `json:"disputed"`
	Rejected      int64  `json:"rejected"`
	Total         int64  `json:"total"`
	AvgConfidence float64 `json:"avg_confidence"`
}

type contribSummary struct {
	GalaxyCount    int     `json:"galaxy_count"`
	TotalAccepted  int64   `json:"total_accepted"`
	TotalPending   int64   `json:"total_pending"`
	TotalDisputed  int64   `json:"total_disputed"`
	TotalRejected  int64   `json:"total_rejected"`
	GlobalAvgConfidence float64 `json:"global_avg_confidence"`
}

// MeContributions returns the active user's contribution stats per galaxy.
// Auth: email session OR wallet session (resolveV4User).
func MeContributions(c *gin.Context) {
	user := resolveV4User(c)
	if user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not logged in"})
		return
	}

	// 单条 SQL 拉出每个 galaxy 的状态分桶 + 平均置信度。
	// COUNT FILTER 是 PostgreSQL 原生语法 — 比若干次 COUNT(CASE WHEN ...) 更短更快。
	rows := []contribRow{}
	err := database.DB.Raw(`
		SELECT
			a.galaxy_id::text                                              AS galaxy_id,
			g.slug                                                         AS galaxy_slug,
			g.title                                                        AS galaxy_title,
			COUNT(*) FILTER (WHERE a.status = ?)                           AS accepted,
			COUNT(*) FILTER (WHERE a.status = ?)                           AS pending,
			COUNT(*) FILTER (WHERE a.status = ?)                           AS disputed,
			COUNT(*) FILTER (WHERE a.status = ?)                           AS rejected,
			COUNT(*)                                                       AS total,
			COALESCE(AVG(a.confidence) FILTER (WHERE a.status = ?), 0)     AS avg_confidence
		FROM atoms a
		JOIN galaxies g ON g.id = a.galaxy_id
		WHERE a.contrib_id = ? AND a.deleted_at IS NULL
		GROUP BY a.galaxy_id, g.slug, g.title
		ORDER BY accepted DESC, total DESC
	`,
		models.AtomStatusAccepted,
		models.AtomStatusPending,
		models.AtomStatusDisputed,
		models.AtomStatusRejected,
		models.AtomStatusAccepted,
		user.ID,
	).Scan(&rows).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	sum := contribSummary{GalaxyCount: len(rows)}
	var weighted float64
	for _, r := range rows {
		sum.TotalAccepted += r.Accepted
		sum.TotalPending += r.Pending
		sum.TotalDisputed += r.Disputed
		sum.TotalRejected += r.Rejected
		weighted += r.AvgConfidence * float64(r.Accepted)
	}
	if sum.TotalAccepted > 0 {
		sum.GlobalAvgConfidence = weighted / float64(sum.TotalAccepted)
	}

	c.JSON(http.StatusOK, gin.H{
		"summary":  sum,
		"galaxies": rows,
	})
}
