package dataengine

import (
	"context"
	"fmt"
	"strings"

	"github.com/MajestaNet/ide/internal/metadata"
)

// allocateAutonumbers assigns next sequence values for autonumber fields on create.
func (s *Service) allocateAutonumbers(ctx context.Context, objectAPIName string, fields []metadata.FieldDefinition, data map[string]any) error {
	q := s.querier(ctx)
	for _, f := range fields {
		if f.FieldType != metadata.FieldTypeAutonumber {
			continue
		}
		var next int64
		err := q.QueryRow(ctx, `
INSERT INTO autonumber_sequences (object_api_name, field_api_name, next_value)
VALUES ($1, $2, COALESCE($3, 1) + 1)
ON CONFLICT (object_api_name, field_api_name) DO UPDATE
SET next_value = autonumber_sequences.next_value + 1
RETURNING next_value - 1`,
			objectAPIName, f.APIName, autonumberStartOrDefault(f),
		).Scan(&next)
		if err != nil {
			return fmt.Errorf("allocate autonumber %s.%s: %w", objectAPIName, f.APIName, err)
		}
		format := "{00000}"
		if f.AutonumberFormat != nil && strings.TrimSpace(*f.AutonumberFormat) != "" {
			format = *f.AutonumberFormat
		}
		data[f.APIName] = formatAutonumber(format, next)
	}
	return nil
}

func autonumberStartOrDefault(f metadata.FieldDefinition) int64 {
	if f.AutonumberStart != nil {
		return int64(*f.AutonumberStart)
	}
	return 1
}

func formatAutonumber(format string, n int64) string {
	// Replace {0+ } width tokens, e.g. A-{00000} → A-00042
	out := format
	start := strings.Index(out, "{")
	for start >= 0 {
		end := strings.Index(out[start:], "}")
		if end < 0 {
			break
		}
		end = start + end
		token := out[start+1 : end]
		width := len(token)
		allZero := width > 0
		for _, r := range token {
			if r != '0' {
				allZero = false
				break
			}
		}
		if allZero {
			repl := fmt.Sprintf("%0*d", width, n)
			out = out[:start] + repl + out[end+1:]
			start = strings.Index(out, "{")
			continue
		}
		start = strings.Index(out[end+1:], "{")
		if start >= 0 {
			start = end + 1 + start
		}
	}
	if out == format && !strings.Contains(format, "{") {
		return fmt.Sprintf("%s%d", format, n)
	}
	return out
}
