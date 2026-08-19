// Package data resolves catalog dataset entries to their actual Parquet
// files on disk and reads structural facts (row count) from them.
package data

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/parquet-go/parquet-go"
)

// RowCount opens the Parquet file for the given dataset name and returns
// its row count, read from the file's footer metadata (no full scan).
func RowCount(dataDir, fileName string) (int64, error) {
	path := filepath.Join(dataDir, fileName)

	f, err := os.Open(path)
	if err != nil {
		return 0, fmt.Errorf("data: open %s: %w", path, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return 0, fmt.Errorf("data: stat %s: %w", path, err)
	}

	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		return 0, fmt.Errorf("data: open parquet file %s: %w", path, err)
	}
	return pf.NumRows(), nil
}
