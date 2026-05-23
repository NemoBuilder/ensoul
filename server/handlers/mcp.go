// Package handlers — MCP-friendly Galaxy 数据导出接口。
//
// 这不是完整的 Model Context Protocol 服务（那需要 stdio transport），而是
// 把 V4 Galaxy 数据以「LLM 应用方便取用」的扁平 JSON 暴露出来，供外部 Vibe
// 应用 / 第三方 MCP server 转包。
//
// GET /api/mcp/galaxy/:slug
//   响应包含: galaxy 元数据 + 全部 accepted nodes + 全部 accepted edges
//   (规模上限 5000 atoms 单次调用，避免拖垮带宽 — 超出走分页 /atoms)
//
// GET /api/mcp/galaxy/:slug/nodes?limit=&cursor=
//   分页节点。cursor = 上一页最后一条 atom.created_at unix。
//
// 故意不在路径前缀加 /v4 — MCP 接口是「面向外部应用」的稳定 API，要独立版本
// 管理。
package handlers

import (
	"net/http"
	"strconv"
	"time"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/gin-gonic/gin"
)

type mcpNode struct {
	ID         string  `json:"id"`
	Label      string  `json:"label"`
	Type       string  `json:"type,omitempty"`
	Summary    string  `json:"summary,omitempty"`
	Confidence float64 `json:"confidence"`
	CreatedAt  int64   `json:"created_at"`
}

type mcpEdge struct {
	ID         string  `json:"id"`
	HeadID     string  `json:"head_id"`
	TailID     string  `json:"tail_id"`
	Label      string  `json:"label"`
	Dir        string  `json:"dir,omitempty"`
	Confidence float64 `json:"confidence"`
	CreatedAt  int64   `json:"created_at"`
}

type mcpGalaxyResp struct {
	Schema   string  `json:"schema"`           // "ensoul.galaxy/v1"
	Galaxy   gin.H   `json:"galaxy"`
	Nodes    []mcpNode `json:"nodes"`
	Edges    []mcpEdge `json:"edges"`
	Truncated bool   `json:"truncated"`
	FetchedAt int64  `json:"fetched_at"`
}

const mcpMaxAtoms = 5000

// MCPGalaxy returns a complete Galaxy snapshot in MCP-friendly form.
func MCPGalaxy(c *gin.Context) {
	slug := c.Param("slug")
	var g models.Galaxy
	if err := database.DB.First(&g, "slug = ?", slug).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
		return
	}

	var atoms []models.Atom
	if err := database.DB.
		Where("galaxy_id = ? AND status = ?", g.ID, models.AtomStatusAccepted).
		Order("created_at ASC, id ASC").
		Limit(mcpMaxAtoms + 1). // 多取一条以判断是否截断
		Find(&atoms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	truncated := len(atoms) > mcpMaxAtoms
	if truncated {
		atoms = atoms[:mcpMaxAtoms]
	}

	nodes := make([]mcpNode, 0, len(atoms))
	edges := make([]mcpEdge, 0)
	for _, a := range atoms {
		switch a.Kind {
		case "node":
			nodes = append(nodes, mcpNode{
				ID:         a.ID.String(),
				Label:      a.NodeLabel,
				Type:       a.NodeType,
				Summary:    a.NodeSummary,
				Confidence: a.Confidence,
				CreatedAt:  a.CreatedAt.Unix(),
			})
		case "edge":
			edge := mcpEdge{
				ID:         a.ID.String(),
				Label:      a.EdgeLabel,
				Dir:        a.EdgeDir,
				Confidence: a.Confidence,
				CreatedAt:  a.CreatedAt.Unix(),
			}
			if a.HeadNodeID != nil {
				edge.HeadID = a.HeadNodeID.String()
			}
			if a.TailNodeID != nil {
				edge.TailID = a.TailNodeID.String()
			}
			edges = append(edges, edge)
		}
	}

	resp := mcpGalaxyResp{
		Schema: "ensoul.galaxy/v1",
		Galaxy: gin.H{
			"id":             g.ID.String(),
			"slug":           g.Slug,
			"title":          g.Title,
			"subtitle":       g.Subtitle,
			"category":       g.Category,
			"lang":           g.Lang,
			"stage":          g.Stage,
			"atom_count":     g.AtomCount,
			"node_count":     g.NodeCount,
			"edge_count":     g.EdgeCount,
			"confidence_avg": g.ConfidenceAvg,
			"nft_token_id":   g.NFTTokenID,
			"token_addr":     g.TokenAddr,
		},
		Nodes:     nodes,
		Edges:     edges,
		Truncated: truncated,
		FetchedAt: time.Now().Unix(),
	}
	c.JSON(http.StatusOK, resp)
}

// MCPGalaxyNodes — 分页节点（cursor = 上一页最后一条 created_at unix 秒）。
func MCPGalaxyNodes(c *gin.Context) {
	slug := c.Param("slug")
	limit, _ := strconv.Atoi(c.Query("limit"))
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	cursorUnix, _ := strconv.ParseInt(c.Query("cursor"), 10, 64)

	var g models.Galaxy
	if err := database.DB.Select("id").First(&g, "slug = ?", slug).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "galaxy not found"})
		return
	}

	q := database.DB.
		Where("galaxy_id = ? AND status = ? AND kind = ?", g.ID, models.AtomStatusAccepted, "node").
		Order("created_at ASC, id ASC").
		Limit(limit + 1)
	if cursorUnix > 0 {
		q = q.Where("created_at > ?", time.Unix(cursorUnix, 0))
	}

	var atoms []models.Atom
	if err := q.Find(&atoms).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	hasMore := len(atoms) > limit
	if hasMore {
		atoms = atoms[:limit]
	}

	out := make([]mcpNode, len(atoms))
	for i, a := range atoms {
		out[i] = mcpNode{
			ID:         a.ID.String(),
			Label:      a.NodeLabel,
			Type:       a.NodeType,
			Summary:    a.NodeSummary,
			Confidence: a.Confidence,
			CreatedAt:  a.CreatedAt.Unix(),
		}
	}
	var nextCursor int64
	if hasMore && len(atoms) > 0 {
		nextCursor = atoms[len(atoms)-1].CreatedAt.Unix()
	}
	c.JSON(http.StatusOK, gin.H{
		"schema":      "ensoul.galaxy.nodes/v1",
		"nodes":       out,
		"next_cursor": nextCursor,
		"has_more":    hasMore,
	})
}

// MCPGalaxyList — 公开 Galaxy 列表（只读，方便外部应用发现）。
func MCPGalaxyList(c *gin.Context) {
	var rows []models.Galaxy
	database.DB.Select("id, slug, title, subtitle, category, lang, stage, atom_count, node_count, edge_count, confidence_avg, token_addr").
		Order("atom_count DESC").
		Limit(200).
		Find(&rows)

	out := make([]gin.H, len(rows))
	for i, g := range rows {
		out[i] = gin.H{
			"id":             g.ID.String(),
			"slug":           g.Slug,
			"title":          g.Title,
			"subtitle":       g.Subtitle,
			"category":       g.Category,
			"lang":           g.Lang,
			"stage":          g.Stage,
			"atom_count":     g.AtomCount,
			"node_count":     g.NodeCount,
			"edge_count":     g.EdgeCount,
			"confidence_avg": g.ConfidenceAvg,
			"token_addr":     g.TokenAddr,
		}
	}
	c.JSON(http.StatusOK, gin.H{
		"schema":   "ensoul.galaxy.list/v1",
		"galaxies": out,
	})
}
