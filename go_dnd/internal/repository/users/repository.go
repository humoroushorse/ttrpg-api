package users

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

func (r *Repository) UpdateUser(ctx context.Context, params UpdateUserParams) (DndUser, error) {
	return New(r.master).UpdateUser(ctx, params)
}

func (r *Repository) GetUserByID(ctx context.Context, id pgtype.UUID) (DndUser, error) {
	return New(r.replica).GetUserByID(ctx, id)
}
