package services

import (
	"fmt"

	"github.com/ensoul-labs/ensoul-server/database"
	"github.com/ensoul-labs/ensoul-server/models"
	"github.com/ensoul-labs/ensoul-server/util"
)

// ──────────────────────────────────────────────────────────────────────────────
// Multi-Dimensional Tag Service — Requirement ②
// Allows filtering Twitter accounts by multiple dimensions (chain, track, role, etc.)
// ──────────────────────────────────────────────────────────────────────────────

// DimensionWithValues is the response type for GET /api/vibe-write/dimensions.
type DimensionWithValues struct {
	models.TagDimension
	Values []models.TagDimensionValue `json:"values"`
}

// GetAllDimensions returns all active dimensions with their values.
func GetAllDimensions() ([]DimensionWithValues, error) {
	var dims []models.TagDimension
	if err := database.DB.Where("active = ?", true).
		Order("sort_order ASC, created_at ASC").
		Find(&dims).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch dimensions: %w", err)
	}

	result := make([]DimensionWithValues, 0, len(dims))
	for _, d := range dims {
		var values []models.TagDimensionValue
		database.DB.Where("dimension_id = ? AND active = ?", d.ID, true).
			Order("sort_order ASC, created_at ASC").
			Find(&values)

		result = append(result, DimensionWithValues{
			TagDimension: d,
			Values:       values,
		})
	}

	return result, nil
}

// GetTagIDsByDimensions returns tag IDs that match ALL the given dimension filters.
// filters is a map of dimension_id → []dimension_value_id.
// A tag must have at least one matching value per dimension to be included (AND across dimensions, OR within).
func GetTagIDsByDimensions(filters map[string][]string) ([]string, error) {
	if len(filters) == 0 {
		return nil, fmt.Errorf("at least one dimension filter is required")
	}

	// Start with all active tag IDs
	var allTagIDs []string
	database.DB.Model(&models.VibeWriteTag{}).Where("active = ?", true).Pluck("id", &allTagIDs)

	if len(allTagIDs) == 0 {
		return nil, nil
	}

	// For each dimension, find matching tags, then intersect
	resultSet := make(map[string]bool)
	for _, id := range allTagIDs {
		resultSet[id] = true
	}

	for _, valueIDs := range filters {
		if len(valueIDs) == 0 {
			continue
		}

		// Find tag IDs that have ANY of the specified dimension values
		var matchingTagIDs []string
		database.DB.Model(&models.VibeWriteTagDimension{}).
			Where("dimension_value_id IN ?", valueIDs).
			Distinct("tag_id").
			Pluck("tag_id", &matchingTagIDs)

		matchSet := make(map[string]bool)
		for _, id := range matchingTagIDs {
			matchSet[id] = true
		}

		// Intersect with current result set
		for id := range resultSet {
			if !matchSet[id] {
				delete(resultSet, id)
			}
		}
	}

	result := make([]string, 0, len(resultSet))
	for id := range resultSet {
		result = append(result, id)
	}

	return result, nil
}

// GetTagDimensions returns the dimension values associated with a tag.
func GetTagDimensions(tagID string) ([]models.TagDimensionValue, error) {
	var associations []models.VibeWriteTagDimension
	if err := database.DB.Where("tag_id = ?", tagID).Find(&associations).Error; err != nil {
		return nil, err
	}

	if len(associations) == 0 {
		return nil, nil
	}

	valueIDs := make([]string, 0, len(associations))
	for _, a := range associations {
		valueIDs = append(valueIDs, a.DimensionValueID)
	}

	var values []models.TagDimensionValue
	database.DB.Where("id IN ?", valueIDs).
		Order("dimension_id ASC, sort_order ASC").
		Find(&values)

	return values, nil
}

// ──────────────────────────────────────────────────────────────────────────────
// Admin: Dimension CRUD
// ──────────────────────────────────────────────────────────────────────────────

// AdminCreateDimension creates a new dimension.
func AdminCreateDimension(dim *models.TagDimension) error {
	if dim.ID == "" {
		return fmt.Errorf("dimension ID is required")
	}
	return database.DB.Create(dim).Error
}

// AdminUpdateDimension updates an existing dimension.
func AdminUpdateDimension(dimID string, updates map[string]interface{}) error {
	result := database.DB.Model(&models.TagDimension{}).Where("id = ?", dimID).Updates(updates)
	if result.RowsAffected == 0 {
		return fmt.Errorf("dimension %s not found", dimID)
	}
	return nil
}

// AdminCreateDimensionValue creates a new dimension value.
func AdminCreateDimensionValue(val *models.TagDimensionValue) error {
	if val.ID == "" || val.DimensionID == "" {
		return fmt.Errorf("value ID and dimension_id are required")
	}

	// Verify dimension exists
	var dim models.TagDimension
	if err := database.DB.First(&dim, "id = ?", val.DimensionID).Error; err != nil {
		return fmt.Errorf("dimension %s not found", val.DimensionID)
	}

	return database.DB.Create(val).Error
}

// AdminUpdateDimensionValue updates an existing dimension value.
func AdminUpdateDimensionValue(valueID string, updates map[string]interface{}) error {
	result := database.DB.Model(&models.TagDimensionValue{}).Where("id = ?", valueID).Updates(updates)
	if result.RowsAffected == 0 {
		return fmt.Errorf("dimension value %s not found", valueID)
	}
	return nil
}

// AdminSetTagDimensions replaces all dimension associations for a tag.
func AdminSetTagDimensions(tagID string, dimensionValueIDs []string) error {
	// Verify tag exists
	var tag models.VibeWriteTag
	if err := database.DB.First(&tag, "id = ?", tagID).Error; err != nil {
		return fmt.Errorf("tag %s not found", tagID)
	}

	// Transaction: delete old + insert new
	tx := database.DB.Begin()

	if err := tx.Where("tag_id = ?", tagID).Delete(&models.VibeWriteTagDimension{}).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to clear old dimensions: %w", err)
	}

	for _, valID := range dimensionValueIDs {
		assoc := models.VibeWriteTagDimension{
			TagID:            tagID,
			DimensionValueID: valID,
		}
		if err := tx.Create(&assoc).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to add dimension value %s: %w", valID, err)
		}
	}

	tx.Commit()
	util.Log.Info("[vibe-write-admin] Set tag %s dimensions: %v", tagID, dimensionValueIDs)
	return nil
}

// AdminListDimensions returns all dimensions (including inactive) for admin.
func AdminListDimensions() ([]DimensionWithValues, error) {
	var dims []models.TagDimension
	if err := database.DB.Order("sort_order ASC, created_at ASC").
		Find(&dims).Error; err != nil {
		return nil, fmt.Errorf("failed to fetch dimensions: %w", err)
	}

	result := make([]DimensionWithValues, 0, len(dims))
	for _, d := range dims {
		var values []models.TagDimensionValue
		database.DB.Where("dimension_id = ?", d.ID).
			Order("sort_order ASC, created_at ASC").
			Find(&values)

		result = append(result, DimensionWithValues{
			TagDimension: d,
			Values:       values,
		})
	}

	return result, nil
}
