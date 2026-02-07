package models

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
)

// JSON is a custom type for JSONB columns in PostgreSQL.
type JSON map[string]interface{}

// Value implements the driver.Valuer interface for database writes.
func (j JSON) Value() (driver.Value, error) {
	if j == nil {
		return "{}", nil
	}
	return json.Marshal(j)
}

// Scan implements the sql.Scanner interface for database reads.
func (j *JSON) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSON)
		return nil
	}

	bytes, ok := value.([]byte)
	if !ok {
		return errors.New("failed to scan JSON: value is not []byte")
	}

	result := make(JSON)
	if err := json.Unmarshal(bytes, &result); err != nil {
		return err
	}
	*j = result
	return nil
}

// DimensionData represents the score and summary for a single dimension.
type DimensionData struct {
	Score   int    `json:"score"`
	Summary string `json:"summary"`
}

// GetDimensions parses the Dimensions JSON into a structured map.
func (s *Shell) GetDimensions() map[string]DimensionData {
	result := make(map[string]DimensionData)
	for key, val := range s.Dimensions {
		if m, ok := val.(map[string]interface{}); ok {
			d := DimensionData{}
			if score, ok := m["score"].(float64); ok {
				d.Score = int(score)
			}
			if summary, ok := m["summary"].(string); ok {
				d.Summary = summary
			}
			result[key] = d
		}
	}
	return result
}
