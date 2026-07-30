package sources

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	master  *pgxpool.Pool
	replica *pgxpool.Pool
}

func NewRepository(master, replica *pgxpool.Pool) *Repository {
	return &Repository{master: master, replica: replica}
}

func (r *Repository) BulkCreateSource(ctx context.Context, params BulkCreateSourceParams) (DndSource, error) {
	return New(r.master).BulkCreateSource(ctx, params)
}

func (r *Repository) QuerySources(ctx context.Context, dndVersion *string) ([]DndSource, error) {
	return New(r.replica).QuerySources(ctx, dndVersion)
}

func (r *Repository) SourceExistsByName(ctx context.Context, name string) (bool, error) {
	return New(r.replica).SourceExistsByName(ctx, name)
}

func (r *Repository) GetSourceByNameVersion(ctx context.Context, name, dndVersion string, dndVersionYear int32) (DndSource, error) {
	return New(r.replica).GetSourceByNameVersion(ctx, GetSourceByNameVersionParams{
		Name:           name,
		DndVersion:     dndVersion,
		DndVersionYear: dndVersionYear,
	})
}

func (r *Repository) GetSourceByNameShortVersion(ctx context.Context, nameShort, dndVersion string, dndVersionYear int32) (DndSource, error) {
	return New(r.replica).GetSourceByNameShortVersion(ctx, GetSourceByNameShortVersionParams{
		Lower:          nameShort,
		DndVersion:     dndVersion,
		DndVersionYear: dndVersionYear,
	})
}
