package tools

import (
	"fmt"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/data"
)

// DatasetSummary is one row of the list_datasets result.
type DatasetSummary struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Origin      string `json:"origin"`
	RowCount    int64  `json:"row_count"`
}

// ListDatasets summarizes every dataset registered in cat, reading each
// one's row count from its Parquet file under dataDir.
func ListDatasets(cat *catalog.Catalog, dataDir string) ([]DatasetSummary, error) {
	summaries := make([]DatasetSummary, 0, len(cat.Datasets))
	for _, ds := range cat.Datasets {
		rows, err := data.RowCount(dataDir, ds.File)
		if err != nil {
			return nil, fmt.Errorf("tools: list datasets: dataset %s: %w", ds.Name, err)
		}
		summaries = append(summaries, DatasetSummary{
			Name:        ds.Name,
			Description: ds.Description,
			Origin:      ds.Origin,
			RowCount:    rows,
		})
	}
	return summaries, nil
}
