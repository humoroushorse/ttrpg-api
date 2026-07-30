package spells

import (
	"context"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Repository struct {
	master  *pgxpool.Pool
	replica *pgxpool.Pool
}

func NewRepository(master, replica *pgxpool.Pool) *Repository {
	return &Repository{master: master, replica: replica}
}

func (r *Repository) CreateSpell(ctx context.Context, params CreateSpellParams) (DndSpell, error) {
	return New(r.master).CreateSpell(ctx, params)
}

func (r *Repository) ListSpells(ctx context.Context, params ListSpellsParams) ([]DndSpell, error) {
	return New(r.replica).ListSpells(ctx, params)
}

func (r *Repository) CountAllSpells(ctx context.Context) (int64, error) {
	return New(r.replica).CountAllSpells(ctx)
}

func (r *Repository) QuerySpells(ctx context.Context, params QuerySpellsParams) ([]DndSpell, error) {
	return New(r.replica).QuerySpells(ctx, params)
}

func (r *Repository) CountQuerySpells(ctx context.Context, params CountQuerySpellsParams) (int64, error) {
	return New(r.replica).CountQuerySpells(ctx, params)
}

func (r *Repository) SpellExistsByNameVersion(ctx context.Context, name, dndVersion string, dndVersionYear int32, sourceID pgtype.UUID) (bool, error) {
	return New(r.master).SpellExistsByNameVersion(ctx, SpellExistsByNameVersionParams{
		Name:           name,
		DndVersion:     dndVersion,
		DndVersionYear: dndVersionYear,
		SourceID:       sourceID,
	})
}

func (r *Repository) GetSpellByUniqueKey(ctx context.Context, name, dndVersion string, dndVersionYear int32, sourceID pgtype.UUID) (DndSpell, error) {
	return New(r.master).GetSpellByUniqueKey(ctx, GetSpellByUniqueKeyParams{
		Name:           name,
		DndVersion:     dndVersion,
		DndVersionYear: dndVersionYear,
		SourceID:       sourceID,
	})
}
