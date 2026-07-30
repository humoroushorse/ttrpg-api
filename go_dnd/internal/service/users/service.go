package users

import (
	"context"
	"errors"

	"github.com/humoroushorse/go_dnd/internal/repository/users"
	"github.com/jackc/pgx/v5/pgtype"
)

var ErrUserNotFound = errors.New("user not found")

type UserService interface {
	GetUser(ctx context.Context, id pgtype.UUID) (users.DndUser, error)
	UpdateUser(ctx context.Context, params users.UpdateUserParams) (users.DndUser, error)
}

type Service struct {
	repo *users.Repository
}

func NewService(repo *users.Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) GetUser(ctx context.Context, id pgtype.UUID) (users.DndUser, error) {
	user, err := s.repo.GetUserByID(ctx, id)
	if err != nil {
		return users.DndUser{}, ErrUserNotFound
	}
	return user, nil
}

func (s *Service) UpdateUser(ctx context.Context, params users.UpdateUserParams) (users.DndUser, error) {
	return s.repo.UpdateUser(ctx, params)
}
