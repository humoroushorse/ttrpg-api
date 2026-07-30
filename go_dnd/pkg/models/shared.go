package models

type GenericListResponse[T any] struct {
	TotalEntitiesCount int            `json:"total_entities_count"`
	Limit              int            `json:"limit"`
	Offset             int            `json:"offset"`
	Filters            map[string]any `json:"filters"`
	EntitiesCount      int            `json:"entities_count"`
	Entities           []T            `json:"entities"`
}

type BulkLoadResponse struct {
	Created  []string `json:"created"`
	Warnings []string `json:"warnings"`
	Errors   []string `json:"errors"`
}
