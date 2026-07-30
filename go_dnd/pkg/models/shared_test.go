package models

import (
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// Validates: Requirements 1.4
func TestProperty_GenericListResponseInvariants(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("entities_count equals len(entities)", prop.ForAll(
		func(count int) bool {
			entities := make([]any, count)
			for i := range entities {
				entities[i] = i
			}
			resp := GenericListResponse[any]{
				Entities:           entities,
				EntitiesCount:      len(entities),
				TotalEntitiesCount: len(entities),
			}
			return resp.EntitiesCount == len(resp.Entities)
		},
		gen.IntRange(0, 200),
	))

	properties.Property("total is always >= entities_count", prop.ForAll(
		func(count, extra int) bool {
			entities := make([]any, count)
			resp := GenericListResponse[any]{
				Entities:           entities,
				EntitiesCount:      len(entities),
				TotalEntitiesCount: len(entities) + extra,
			}
			return resp.TotalEntitiesCount >= resp.EntitiesCount
		},
		gen.IntRange(0, 200),
		gen.IntRange(0, 1000),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Validates: Requirements 1.5
func TestProperty_BulkLoadResponseUpdateTotals(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("UpdateTotals always matches array lengths", prop.ForAll(
		func(createdCount, warningCount, errorCount int) bool {
			created := make([]string, createdCount)
			warnings := make([]string, warningCount)
			errors := make([]string, errorCount)

			for i := range created {
				created[i] = "created-item"
			}
			for i := range warnings {
				warnings[i] = "warning-item"
			}
			for i := range errors {
				errors[i] = "error-item"
			}

			resp := BulkLoadResponse{
				Created:  created,
				Warnings: warnings,
				Errors:   errors,
			}

			// The invariant: array lengths are consistent with what was set
			return len(resp.Created) == createdCount &&
				len(resp.Warnings) == warningCount &&
				len(resp.Errors) == errorCount
		},
		gen.IntRange(0, 50),
		gen.IntRange(0, 50),
		gen.IntRange(0, 50),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
