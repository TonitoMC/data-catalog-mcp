package data

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/parquet-go/parquet-go"
)

// ErrColumnNotFound is returned by ReadColumn when the named column isn't
// in the file's schema.
var ErrColumnNotFound = errors.New("data: column not found")

// openParquetFile opens the Parquet file for the given dataset file name
// under dataDir. The caller must close the returned file once done with pf.
func openParquetFile(dataDir, fileName string) (*os.File, *parquet.File, error) {
	path := filepath.Join(dataDir, fileName)

	f, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("data: open %s: %w", path, err)
	}

	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("data: stat %s: %w", path, err)
	}

	pf, err := parquet.OpenFile(f, info.Size())
	if err != nil {
		f.Close()
		return nil, nil, fmt.Errorf("data: open parquet file %s: %w", path, err)
	}
	return f, pf, nil
}

// Columns returns the leaf column names declared in the Parquet file's
// schema, in file order. Nested paths (there are none in this catalog's
// datasets, but Parquet allows them) are dot-joined.
func Columns(dataDir, fileName string) ([]string, error) {
	f, pf, err := openParquetFile(dataDir, fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	paths := pf.Schema().Columns()
	names := make([]string, len(paths))
	for i, p := range paths {
		names[i] = strings.Join(p, ".")
	}
	return names, nil
}

// Cell is one value read from a column: either Null, or Value holding a
// Go-native bool/int64/float64/string.
type Cell struct {
	Null  bool
	Value any
}

// ReadColumn reads every value of the named column, top to bottom.
func ReadColumn(dataDir, fileName, column string) ([]Cell, error) {
	f, pf, err := openParquetFile(dataDir, fileName)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	leaf, ok := pf.Schema().Lookup(column)
	if !ok {
		return nil, fmt.Errorf("%w: %s: %s", ErrColumnNotFound, fileName, column)
	}

	reader := parquet.NewReader(pf)
	defer reader.Close()

	rows := make([]parquet.Row, 256)
	cells := make([]Cell, 0, pf.NumRows())
	for {
		n, err := reader.ReadRows(rows)
		for _, row := range rows[:n] {
			cells = append(cells, valueToCell(row[leaf.ColumnIndex]))
		}
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("data: read column %q from %s: %w", column, fileName, err)
		}
	}
	return cells, nil
}

// valueToCell converts a parquet.Value into a Cell holding a plain Go type.
func valueToCell(v parquet.Value) Cell {
	if v.IsNull() {
		return Cell{Null: true}
	}
	switch v.Kind() {
	case parquet.Boolean:
		return Cell{Value: v.Boolean()}
	case parquet.Int32:
		return Cell{Value: int64(v.Int32())}
	case parquet.Int64:
		return Cell{Value: v.Int64()}
	case parquet.Float:
		return Cell{Value: float64(v.Float())}
	case parquet.Double:
		return Cell{Value: v.Double()}
	case parquet.ByteArray, parquet.FixedLenByteArray:
		return Cell{Value: string(v.ByteArray())}
	default:
		return Cell{Value: v.String()}
	}
}
