package tools

import (
	"context"
	"log"
	"math"
	"sort"
	"strings"
	"sync"

	"github.com/tonitomc/data-catalog-mcp/internal/catalog"
	"github.com/tonitomc/data-catalog-mcp/internal/embeddings"
)

// SearchResult is one hit: either a dataset (Column empty) or a column
// within one, ranked by relevance to the query.
type SearchResult struct {
	Dataset     string  `json:"dataset"`
	Column      string  `json:"column,omitempty"`
	Description string  `json:"description"`
	Score       float64 `json:"score"`
}

// defaultSearchLimit bounds how many results SearchCatalog returns when
// the caller doesn't ask for a specific number.
const defaultSearchLimit = 5

// searchDoc is one embeddable/searchable unit of catalog metadata.
type searchDoc struct {
	dataset, column, text string
}

// docVectorCache holds every catalog doc's embedding, computed once per
// process and reused for every subsequent search_catalog call — the
// catalog's metadata doesn't change while the process is running, so
// there's no reason to re-embed it on every request. It's plain
// in-memory state: nothing persists across restarts, and a restart is
// exactly how you'd force it to recompute (e.g. after editing
// catalog.yaml) — there's no separate "reset" mechanism because the
// cache's entire lifetime is the process's lifetime.
type docVectorCache struct {
	once    sync.Once
	err     error
	entries []cachedDoc
}

type cachedDoc struct {
	doc searchDoc
	vec []float32
}

var globalDocCache docVectorCache

// SearchCatalog finds the datasets/columns whose catalog metadata is most
// relevant to query. If embed is non-nil, it's used for semantic
// (embedding-similarity) search; on any embeddings failure (including
// embed being nil, e.g. no service configured), it falls back to keyword
// matching over dataset/column names and descriptions.
func SearchCatalog(ctx context.Context, cat *catalog.Catalog, embed embeddings.Client, query string, limit int) ([]SearchResult, error) {
	if limit <= 0 {
		limit = defaultSearchLimit
	}
	docs := buildSearchDocs(cat)

	if embed != nil {
		results, err := searchByEmbedding(ctx, embed, docs, query, limit)
		if err == nil {
			return results, nil
		}
		log.Printf("tools: search_catalog: embeddings unavailable, falling back to keyword search: %v", err)
	}
	return searchByKeyword(docs, query, limit), nil
}

func buildSearchDocs(cat *catalog.Catalog) []searchDoc {
	docs := make([]searchDoc, 0, len(cat.Datasets))
	for _, ds := range cat.Datasets {
		docs = append(docs, searchDoc{dataset: ds.Name, text: ds.Name + ": " + ds.Description})
		for _, col := range ds.Columns {
			docs = append(docs, searchDoc{dataset: ds.Name, column: col.Name, text: col.Name + ": " + col.Description})
		}
	}
	return docs
}

// cachedDocVectors returns every doc's embedding, computing them all
// (once, ever, per process) on the first call and reusing that result
// for every call after.
func cachedDocVectors(ctx context.Context, embed embeddings.Client, docs []searchDoc) ([]cachedDoc, error) {
	globalDocCache.once.Do(func() {
		entries := make([]cachedDoc, 0, len(docs))
		for _, d := range docs {
			vec, err := embed.EmbedDocument(ctx, d.text)
			if err != nil {
				globalDocCache.err = err
				return
			}
			entries = append(entries, cachedDoc{doc: d, vec: vec})
		}
		globalDocCache.entries = entries
	})
	return globalDocCache.entries, globalDocCache.err
}

func searchByEmbedding(ctx context.Context, embed embeddings.Client, docs []searchDoc, query string, limit int) ([]SearchResult, error) {
	queryVec, err := embed.EmbedQuery(ctx, query)
	if err != nil {
		return nil, err
	}

	cached, err := cachedDocVectors(ctx, embed, docs)
	if err != nil {
		return nil, err
	}

	results := make([]SearchResult, 0, len(cached))
	for _, c := range cached {
		results = append(results, SearchResult{
			Dataset: c.doc.dataset, Column: c.doc.column, Description: c.doc.text,
			Score: cosineSimilarity(queryVec, c.vec),
		})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > limit {
		results = results[:limit]
	}
	return results, nil
}

func searchByKeyword(docs []searchDoc, query string, limit int) []SearchResult {
	terms := strings.Fields(strings.ToLower(query))

	results := make([]SearchResult, 0, len(docs))
	for _, d := range docs {
		text := strings.ToLower(d.text)
		matches := 0
		for _, term := range terms {
			if strings.Contains(text, term) {
				matches++
			}
		}
		if matches == 0 {
			continue
		}
		results = append(results, SearchResult{
			Dataset: d.dataset, Column: d.column, Description: d.text,
			Score: float64(matches) / float64(len(terms)),
		})
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Score > results[j].Score })
	if len(results) > limit {
		results = results[:limit]
	}
	return results
}

func cosineSimilarity(a, b []float32) float64 {
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
