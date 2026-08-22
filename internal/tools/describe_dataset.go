package tools

import (
	"fmt"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/data"
)

// ColumnDetail is one column entry in a DatasetDetail.
type ColumnDetail struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Unit        string `json:"unit,omitempty"`
	RuleCount   int    `json:"rule_count"`
}

// DatasetDetail is the describe_dataset result: the full data dictionary
// entry for one dataset plus its live row count.
type DatasetDetail struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Origin      string         `json:"origin"`
	RowCount    int64          `json:"row_count"`
	Columns     []ColumnDetail `json:"columns"`
}

// DescribeDataset returns the full catalog entry for name, enriched with
// its live row count. Returns ErrDatasetNotFound if name isn't registered.
func DescribeDataset(cat *catalog.Catalog, dataDir, name string) (*DatasetDetail, error) {
	ds, ok := cat.Dataset(name)
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrDatasetNotFound, name)
	}

	rows, err := data.RowCount(dataDir, ds.File)
	if err != nil {
		return nil, fmt.Errorf("tools: describe dataset %s: %w", name, err)
	}

	columns := make([]ColumnDetail, 0, len(ds.Columns))
	for _, col := range ds.Columns {
		columns = append(columns, ColumnDetail{
			Name:        col.Name,
			Type:        col.Type,
			Description: col.Description,
			Unit:        col.Unit,
			RuleCount:   len(col.Rules),
		})
	}

	return &DatasetDetail{
		Name:        ds.Name,
		Description: ds.Description,
		Origin:      ds.Origin,
		RowCount:    rows,
		Columns:     columns,
	}, nil
}
