package search

import (
	"encoding/base64"
	"encoding/json"
)

// buildQuery translates Criteria into an OpenSearch query.
func buildQuery(criteria Criteria) map[string]any {
	hasFilters := len(criteria.Filters) > 0
	hasMatches := len(criteria.Matches) > 0
	hasVector := criteria.Vector != nil

	if !hasFilters && !hasMatches && !hasVector {
		return map[string]any{"match_all": map[string]any{}}
	}

	// When we have vector + matches, use hybrid query
	if hasVector && hasMatches {
		return buildHybridQuery(criteria)
	}

	// When we have only vector, use knn query
	if hasVector {
		knnField := map[string]any{
			"vector": criteria.Vector.Values,
			"k":      criteria.Vector.K,
		}
		if hasFilters {
			knnField["filter"] = map[string]any{
				"bool": map[string]any{
					"filter": buildFilterClauses(criteria.Filters),
				},
			}
		}
		return map[string]any{
			"knn": map[string]any{
				criteria.Vector.Field: knnField,
			},
		}
	}

	// Standard bool query for filters + matches
	boolQuery := map[string]any{}
	if hasFilters {
		boolQuery["filter"] = buildFilterClauses(criteria.Filters)
	}
	if hasMatches {
		boolQuery["must"] = buildMatchClauses(criteria.Matches)
	}

	return map[string]any{"bool": boolQuery}
}

// buildHybridQuery creates an OpenSearch hybrid query combining text and vector search.
func buildHybridQuery(criteria Criteria) map[string]any {
	boolQuery := map[string]any{}
	if len(criteria.Filters) > 0 {
		boolQuery["filter"] = buildFilterClauses(criteria.Filters)
	}
	boolQuery["must"] = buildMatchClauses(criteria.Matches)

	return map[string]any{
		"hybrid": map[string]any{
			"queries": []map[string]any{
				{"bool": boolQuery},
				{
					"knn": map[string]any{
						criteria.Vector.Field: map[string]any{
							"vector": criteria.Vector.Values,
							"k":      criteria.Vector.K,
						},
					},
				},
			},
		},
	}
}

func buildFilterClauses(filters []Filter) []map[string]any {
	clauses := make([]map[string]any, len(filters))
	for i, f := range filters {
		clauses[i] = map[string]any{
			"term": map[string]any{
				f.Field: f.Value,
			},
		}
	}
	return clauses
}

func buildMatchClauses(matches []Match) []map[string]any {
	clauses := make([]map[string]any, len(matches))
	for i, m := range matches {
		clauses[i] = map[string]any{
			"match": map[string]any{
				m.Field: m.Query,
			},
		}
	}
	return clauses
}

func encodeSearchAfter(sortValues []any) string {
	data, _ := json.Marshal(sortValues)
	return base64.StdEncoding.EncodeToString(data)
}

func decodeSearchAfter(token string) ([]any, error) {
	data, err := base64.StdEncoding.DecodeString(token)
	if err != nil {
		return nil, err
	}
	var sortValues []any
	if err := json.Unmarshal(data, &sortValues); err != nil {
		return nil, err
	}
	return sortValues, nil
}
