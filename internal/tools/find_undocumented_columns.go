package tools

import (
	"fmt"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/data"
)

// UndocumentedColumns lists the columns one dataset's Parquet file has that
// aren't declared in the catalog's data dictionary for it.
type UndocumentedColumns struct {
	Dataset string   `json:"dataset"`
	Columns []string `json:"undocumented_columns"`
}

// FindUndocumentedColumns compares each dataset's actual Parquet schema
// against its catalog entry. If dataset is non-empty, only that dataset is
// checked; otherwise every registered dataset is. Datasets with no
// undocumented columns are omitted from the result, so an empty result
// means the catalog is fully in sync with the data.
func FindUndocumentedColumns(cat *catalog.Catalog, dataDir, dataset string) ([]UndocumentedColumns, error) {
	targets := cat.Datasets
	if dataset != "" {
		ds, ok := cat.Dataset(dataset)
		if !ok {
			return nil, fmt.Errorf("%w: %s", ErrDatasetNotFound, dataset)
		}
		targets = []catalog.Dataset{*ds}
	}

	results := make([]UndocumentedColumns, 0, len(targets))
	for _, ds := range targets {
		actual, err := data.Columns(dataDir, ds.File)
		if err != nil {
			return nil, fmt.Errorf("tools: find undocumented columns: dataset %s: %w", ds.Name, err)
		}

		documented := make(map[string]struct{}, len(ds.Columns))
		for _, col := range ds.Columns {
			documented[col.Name] = struct{}{}
		}

		var undocumented []string
		for _, name := range actual {
			if _, ok := documented[name]; !ok {
				undocumented = append(undocumented, name)
			}
		}
		if len(undocumented) > 0 {
			results = append(results, UndocumentedColumns{Dataset: ds.Name, Columns: undocumented})
		}
	}
	return results, nil
}
