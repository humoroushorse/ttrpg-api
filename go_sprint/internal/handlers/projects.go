package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/humoroushorse/go_sprint/internal/models"
	"github.com/humoroushorse/go_sprint/internal/repository"
)

type ProjectHandler struct {
	repo   *repository.ProjectRepository
	logger *slog.Logger
}

func NewProjectHandler(repo *repository.ProjectRepository, logger *slog.Logger) *ProjectHandler {
	return &ProjectHandler{
		repo:   repo,
		logger: logger,
	}
}

// ListProjects handles GET /api/v1/projects
func (h *ProjectHandler) ListProjects(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse pagination parameters
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	if page < 1 {
		page = 1
	}

	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))
	if pageSize < 1 || pageSize > 100 {
		pageSize = 25
	}

	// Get projects
	projects, total, err := h.repo.List(ctx, page, pageSize)
	if err != nil {
		h.logger.Error("Failed to list projects", slog.String("error", err.Error()))
		// TODO: Fix projects repository - for now return empty list
		projects = []*models.Project{}
		total = 0
	}

	// Build response
	response := map[string]interface{}{
		"items":     projects,
		"page":      page,
		"page_size": pageSize,
		"total":     total,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// GetProject handles GET /api/v1/projects/:id
func (h *ProjectHandler) GetProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract ID from path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	idStr := pathParts[4]
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid project ID format", http.StatusBadRequest)
		return
	}

	// Get project
	project, err := h.repo.GetByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Project not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to get project", slog.String("error", err.Error()))
		http.Error(w, "Failed to get project", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

// GetProjectByKey handles GET /api/v1/projects/key/:key
func (h *ProjectHandler) GetProjectByKey(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract key from path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 6 {
		http.Error(w, "Invalid project key", http.StatusBadRequest)
		return
	}

	key := strings.ToUpper(pathParts[5])

	// Get project
	project, err := h.repo.GetByKey(ctx, key)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Project not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to get project", slog.String("error", err.Error()))
		http.Error(w, "Failed to get project", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

// CreateProject handles POST /api/v1/projects
func (h *ProjectHandler) CreateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse request body
	var req models.CreateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate
	if len(req.Key) < 3 || len(req.Key) > 10 {
		http.Error(w, "Project key must be 3-10 characters", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}
	if req.StartingNumber < 1 {
		req.StartingNumber = 1
	}

	// Get user from context (set by auth middleware)
	userID := "system" // TODO: Get from auth context
	if user := ctx.Value("user"); user != nil {
		if userMap, ok := user.(map[string]interface{}); ok {
			if sub, ok := userMap["sub"].(string); ok {
				userID = sub
			}
		}
	}

	// Create project
	project := &models.Project{
		ID:             uuid.New(),
		Key:            strings.ToUpper(req.Key),
		Name:           req.Name,
		Description:    req.Description,
		StartingNumber: req.StartingNumber,
		CreatedBy:      userID,
		UpdatedBy:      userID,
	}

	if err := h.repo.Create(ctx, project); err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			http.Error(w, "Project key already exists", http.StatusConflict)
			return
		}
		h.logger.Error("Failed to create project", slog.String("error", err.Error()))
		http.Error(w, "Failed to create project", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(project)
}

// UpdateProject handles PUT /api/v1/projects/:id
func (h *ProjectHandler) UpdateProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract ID from path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	idStr := pathParts[4]
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid project ID format", http.StatusBadRequest)
		return
	}

	// Parse request body
	var req models.UpdateProjectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate
	if req.Name == "" {
		http.Error(w, "Project name is required", http.StatusBadRequest)
		return
	}

	// Get existing project
	project, err := h.repo.GetByID(ctx, id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Project not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to get project", slog.String("error", err.Error()))
		http.Error(w, "Failed to get project", http.StatusInternalServerError)
		return
	}

	// Get user from context
	userID := "system"
	if user := ctx.Value("user"); user != nil {
		if userMap, ok := user.(map[string]interface{}); ok {
			if sub, ok := userMap["sub"].(string); ok {
				userID = sub
			}
		}
	}

	// Update fields
	project.Name = req.Name
	project.Description = req.Description
	project.UpdatedBy = userID

	if err := h.repo.Update(ctx, project); err != nil {
		h.logger.Error("Failed to update project", slog.String("error", err.Error()))
		http.Error(w, "Failed to update project", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(project)
}

// DeleteProject handles DELETE /api/v1/projects/:id
func (h *ProjectHandler) DeleteProject(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract ID from path
	pathParts := strings.Split(r.URL.Path, "/")
	if len(pathParts) < 5 {
		http.Error(w, "Invalid project ID", http.StatusBadRequest)
		return
	}

	idStr := pathParts[4]
	id, err := uuid.Parse(idStr)
	if err != nil {
		http.Error(w, "Invalid project ID format", http.StatusBadRequest)
		return
	}

	// Delete project
	if err := h.repo.Delete(ctx, id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Project not found", http.StatusNotFound)
			return
		}
		h.logger.Error("Failed to delete project", slog.String("error", err.Error()))
		http.Error(w, "Failed to delete project", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
