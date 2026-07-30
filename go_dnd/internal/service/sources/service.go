package sources

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_dnd/internal/repository/sources"
	"github.com/humoroushorse/go_dnd/pkg/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

var (
	ErrSourceNotFound = errors.New("source not found")
	ErrRequiredField  = errors.New("required field is missing")
)

type SourceService interface {
	QuerySources(ctx context.Context, dndVersion *string) ([]sources.DndSource, error)
	BulkLoadSources(ctx context.Context, records []CreateSourceInput, userID uuid.UUID) (models.BulkLoadResponse, error)
}

type Service struct {
	repo *sources.Repository
}

func NewService(repo *sources.Repository) *Service {
	return &Service{repo: repo}
}

type CreateSourceInput struct {
	ID             string `json:"id,omitempty"`
	Name           string `json:"name"`
	NameShort      string `json:"name_short"`
	PublishYear    *int32 `json:"publish_year,omitempty"`
	DndVersion     string `json:"dnd_version"`
	DndVersionYear int32  `json:"dnd_version_year,omitempty"`
}

func (s *Service) QuerySources(ctx context.Context, dndVersion *string) ([]sources.DndSource, error) {
	return s.repo.QuerySources(ctx, dndVersion)
}

// BulkLoadSources validates all records first — no commit on any parse error.
// Duplicate names produce a warning and are skipped rather than erroring.
func (s *Service) BulkLoadSources(ctx context.Context, records []CreateSourceInput, userID uuid.UUID) (models.BulkLoadResponse, error) {
	result := models.BulkLoadResponse{}

	for i, rec := range records {
		if err := validateSourceInput(rec); err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("record %d (%s): %v", i, rec.Name, err))
		}
	}
	if len(result.Errors) > 0 {
		return result, nil
	}

	now := pgtype.Timestamptz{Time: time.Now().UTC(), Valid: true}
	createdBy := pgtype.UUID{Bytes: userID, Valid: true}

	for _, rec := range records {
		_, err := s.repo.GetSourceByNameVersion(ctx, rec.Name, rec.DndVersion, rec.DndVersionYear)
		if err == nil {
			// Source already exists with same name+version+year
			result.Warnings = append(result.Warnings, fmt.Sprintf("source %q (%s %d) already exists, skipping", rec.Name, rec.DndVersion, rec.DndVersionYear))
			continue
		}
		if !errors.Is(err, pgx.ErrNoRows) {
			// Actual DB error
			result.Errors = append(result.Errors, fmt.Sprintf("checking source %q: %v", rec.Name, err))
			return result, nil
		}

		id := rec.ID
		if id == "" {
			id = uuid.New().String()
		}
		parsedID, err := uuid.Parse(id)
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid id for source %q: %v", rec.Name, err))
			return result, nil
		}

		src, err := s.repo.BulkCreateSource(ctx, sources.BulkCreateSourceParams{
			ID:             pgtype.UUID{Bytes: parsedID, Valid: true},
			Name:           rec.Name,
			NameShort:      rec.NameShort,
			PublishYear:    rec.PublishYear,
			DndVersion:     rec.DndVersion,
			DndVersionYear: rec.DndVersionYear,
			CreatedAt:      now,
			UpdatedAt:      now,
			CreatedBy:      createdBy,
			UpdatedBy:      createdBy,
		})
		if err != nil {
			result.Errors = append(result.Errors, fmt.Sprintf("creating source %q: %v", rec.Name, err))
			return result, nil
		}
		result.Created = append(result.Created, fmt.Sprintf("%s (%s %d)", src.Name, src.DndVersion, src.DndVersionYear))
	}

	return result, nil
}

func validateSourceInput(input CreateSourceInput) error {
	if input.Name == "" {
		return fmt.Errorf("%w: name", ErrRequiredField)
	}
	if input.NameShort == "" {
		return fmt.Errorf("%w: name_short", ErrRequiredField)
	}
	if input.DndVersion == "" {
		return fmt.Errorf("%w: dnd_version", ErrRequiredField)
	}
	return nil
}
