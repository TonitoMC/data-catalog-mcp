// Package data resolves catalog dataset entries to their actual Parquet
// files on disk and reads structural facts (row count, schema, column
// values) from them.
package data

// RowCount opens the Parquet file for the given dataset name and returns
// its row count, read from the file's footer metadata (no full scan).
func RowCount(dataDir, fileName string) (int64, error) {
	f, pf, err := openParquetFile(dataDir, fileName)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	return pf.NumRows(), nil
}
