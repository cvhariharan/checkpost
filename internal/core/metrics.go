package core

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/cvhariharan/checkpost/internal/models"
	"github.com/google/uuid"
	"github.com/invopop/jsonschema"
)

// GetNodeMetrics returns the current device-metric snapshots for one
// node, keyed by metric kind. Kinds with no data yet are absent from
// the map.
func (c *Core) GetNodeMetrics(ctx context.Context, req models.NodeIdentity) (map[string]models.NodeMetric, error) {
	node, err := c.GetNodeByID(ctx, req)
	if err != nil {
		return nil, err
	}

	nodeUUID, err := uuid.Parse(node.UUID)
	if err != nil {
		return nil, fmt.Errorf("parse node uuid: %w", err)
	}

	rows, err := c.store.ListNodeMetricsByNodeUUID(ctx, nodeUUID)
	if err != nil {
		return nil, fmt.Errorf("list node metrics: %w", err)
	}

	out := make(map[string]models.NodeMetric, len(rows))
	for _, row := range rows {
		var value interface{}
		if err := json.Unmarshal(row.Value, &value); err != nil {
			return nil, fmt.Errorf("unmarshal metric %q: %w", row.Kind, err)
		}
		out[row.Kind] = models.NodeMetric{
			Kind:        row.Kind,
			Value:       value,
			CollectedAt: row.CollectedAt,
			UpdatedAt:   row.UpdatedAt,
		}
	}
	return out, nil
}

// MetricSchemas returns the kind → JSON Schema map plus the canonical
// render order. The result is read-only and safe to share.
type MetricSchemas struct {
	Schemas map[Kind]*jsonschema.Schema `json:"schemas"`
	Kinds   []Kind                      `json:"kinds"`
}

func (c *Core) GetMetricSchemas() MetricSchemas {
	return MetricSchemas{
		Schemas: c.systemMetrics.Schemas(),
		Kinds:   c.systemMetrics.Kinds(),
	}
}
