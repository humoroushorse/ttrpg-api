package repository

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ProjectRepository struct {
	master  *pgxpool.Pool
	replica *pgxpool.Pool
}

func NewProjectRepository(master, replica *pgxpool.Pool) *ProjectRepository {
	return &ProjectRepository{
		master:  master,
		replica: replica,
	}
}

// Create creates a new project
func (r *ProjectRepository) Create(ctx context.Context, project *models.Project) error {
	query := `
		INSERT INTO sprint_management.projects (id, key, name, description, starting_number, current_counter, created_by, updated_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		RETURNING created_at, updated_at
	`

	// Ensure key is uppercase
	project.Key = strings.ToUpper(project.Key)

	// Set current_counter to starting_number - 1 (so first ticket gets starting_number)
	project.CurrentCounter = project.StartingNumber - 1

	err := r.master.QueryRow(
		ctx,
		query,
		project.ID,
		project.Key,
		project.Name,
		project.Description,
		project.StartingNumber,
		project.CurrentCounter,
		project.CreatedBy,
		project.UpdatedBy,
	).Scan(&project.CreatedAt, &project.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	return nil
}

// GetByID retrieves a project by ID
func (r *ProjectRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Project, error) {
	query := `
		SELECT id, key, name, description, current_counter, starting_number,
		       created_by, created_at, updated_by, updated_at
		FROM sprint_management.projects
		WHERE id = $1
	`

	project := &models.Project{}
	err := r.replica.QueryRow(ctx, query, id).Scan(
		&project.ID,
		&project.Key,
		&project.Name,
		&project.Description,
		&project.CurrentCounter,
		&project.StartingNumber,
		&project.CreatedBy,
		&project.CreatedAt,
		&project.UpdatedBy,
		&project.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("project not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return project, nil
}

// GetNextTicketNumber atomically increments and returns the next ticket number for a project
func (r *ProjectRepository) GetNextTicketNumber(ctx context.Context, projectID uuid.UUID) (int, error) {
	query := `
		UPDATE sprint_management.projects
		SET current_counter = current_counter + 1
		WHERE id = $1
		RETURNING current_counter
	`

	var ticketNumber int
	err := r.master.QueryRow(ctx, query, projectID).Scan(&ticketNumber)
	if err == pgx.ErrNoRows {
		return 0, fmt.Errorf("project not found")
	}
	if err != nil {
		return 0, fmt.Errorf("failed to get next ticket number: %w", err)
	}

	return ticketNumber, nil
}

// GetByKey retrieves a project by key
func (r *ProjectRepository) GetByKey(ctx context.Context, key string) (*models.Project, error) {
	query := `
		SELECT id, key, name, description, current_counter, starting_number,
		       created_by, created_at, updated_by, updated_at
		FROM sprint_management.projects
		WHERE key = $1
	`

	project := &models.Project{}
	err := r.replica.QueryRow(ctx, query, strings.ToUpper(key)).Scan(
		&project.ID,
		&project.Key,
		&project.Name,
		&project.Description,
		&project.CurrentCounter,
		&project.StartingNumber,
		&project.CreatedBy,
		&project.CreatedAt,
		&project.UpdatedBy,
		&project.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("project not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return project, nil
}

// List retrieves all projects with pagination
func (r *ProjectRepository) List(ctx context.Context, page, pageSize int) ([]*models.Project, int, error) {
	// Get total count
	var total int
	err := r.replica.QueryRow(ctx, "SELECT COUNT(*) FROM sprint_management.projects").Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to count projects: %w", err)
	}

	// Get paginated results
	query := `
		SELECT id, key, name, description, current_counter, starting_number,
		       created_by, created_at, updated_by, updated_at
		FROM sprint_management.projects
		ORDER BY created_at DESC
		LIMIT $1 OFFSET $2
	`

	offset := (page - 1) * pageSize
	rows, err := r.replica.Query(ctx, query, pageSize, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to list projects: %w", err)
	}
	defer rows.Close()

	projects := make([]*models.Project, 0)
	for rows.Next() {
		project := &models.Project{}
		err := rows.Scan(
			&project.ID,
			&project.Key,
			&project.Name,
			&project.Description,
			&project.CurrentCounter,
			&project.StartingNumber,
			&project.CreatedBy,
			&project.CreatedAt,
			&project.UpdatedBy,
			&project.UpdatedAt,
		)
		if err != nil {
			return nil, 0, fmt.Errorf("failed to scan project: %w", err)
		}
		projects = append(projects, project)
	}

	return projects, total, nil
}

// Update updates a project
func (r *ProjectRepository) Update(ctx context.Context, project *models.Project) error {
	query := `
		UPDATE sprint_management.projects
		SET name = $1, description = $2, updated_by = $3, updated_at = NOW()
		WHERE id = $4
		RETURNING updated_at
	`

	err := r.master.QueryRow(
		ctx,
		query,
		project.Name,
		project.Description,
		project.UpdatedBy,
		project.ID,
	).Scan(&project.UpdatedAt)

	if err == pgx.ErrNoRows {
		return fmt.Errorf("project not found")
	}
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
	}

	return nil
}

// Delete deletes a project
func (r *ProjectRepository) Delete(ctx context.Context, id uuid.UUID) error {
	query := "DELETE FROM sprint_management.projects WHERE id = $1"

	result, err := r.master.Exec(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("project not found")
	}

	return nil
}
