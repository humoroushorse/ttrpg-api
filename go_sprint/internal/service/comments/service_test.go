package comments

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"

	"github.com/humoroushorse/go_sprint/internal/repository/comments"
)

// newTestLogger creates a logger for testing
func newTestLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelError, // Only show errors in tests
	}))
}

// Mock repository for testing
type mockRepository struct {
	comments     map[uuid.UUID]comments.SprintManagementComment
	activityLogs []comments.SprintManagementActivityLog
}

func newMockRepository() *mockRepository {
	return &mockRepository{
		comments:     make(map[uuid.UUID]comments.SprintManagementComment),
		activityLogs: make([]comments.SprintManagementActivityLog, 0),
	}
}

func (m *mockRepository) CreateComment(ctx context.Context, params comments.CreateCommentParams) (comments.SprintManagementComment, error) {
	id := uuid.New()
	comment := comments.SprintManagementComment{
		ID:         uuidToPgtype(id),
		WorkItemID: params.WorkItemID,
		AuthorID:   params.AuthorID,
		Content:    params.Content,
		CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
		UpdatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.comments[id] = comment
	return comment, nil
}

func (m *mockRepository) GetCommentByID(ctx context.Context, id pgtype.UUID) (comments.SprintManagementComment, error) {
	comment, ok := m.comments[pgtypeToUUID(id)]
	if !ok || comment.DeletedAt.Valid {
		return comments.SprintManagementComment{}, ErrCommentNotFound
	}
	return comment, nil
}

func (m *mockRepository) UpdateComment(ctx context.Context, params comments.UpdateCommentParams) (comments.SprintManagementComment, error) {
	comment, ok := m.comments[pgtypeToUUID(params.ID)]
	if !ok || comment.DeletedAt.Valid {
		return comments.SprintManagementComment{}, ErrCommentNotFound
	}
	comment.Content = params.Content
	comment.UpdatedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	m.comments[pgtypeToUUID(params.ID)] = comment
	return comment, nil
}

func (m *mockRepository) SoftDeleteComment(ctx context.Context, params comments.SoftDeleteCommentParams) error {
	comment, ok := m.comments[pgtypeToUUID(params.ID)]
	if !ok {
		return ErrCommentNotFound
	}
	comment.DeletedAt = pgtype.Timestamptz{Time: time.Now(), Valid: true}
	comment.DeletedBy = params.DeletedBy
	m.comments[pgtypeToUUID(params.ID)] = comment
	return nil
}

func (m *mockRepository) RestoreComment(ctx context.Context, id pgtype.UUID) error {
	comment, ok := m.comments[pgtypeToUUID(id)]
	if !ok {
		return ErrCommentNotFound
	}
	comment.DeletedAt = pgtype.Timestamptz{Valid: false}
	comment.DeletedBy = pgtype.UUID{Valid: false}
	m.comments[pgtypeToUUID(id)] = comment
	return nil
}

func (m *mockRepository) ListCommentsByWorkItem(ctx context.Context, workItemID pgtype.UUID) ([]comments.SprintManagementComment, error) {
	result := make([]comments.SprintManagementComment, 0)
	for _, comment := range m.comments {
		if comment.WorkItemID == workItemID && !comment.DeletedAt.Valid {
			result = append(result, comment)
		}
	}
	return result, nil
}

func (m *mockRepository) CountCommentsByWorkItem(ctx context.Context, workItemID pgtype.UUID) (int64, error) {
	count := int64(0)
	for _, comment := range m.comments {
		if comment.WorkItemID == workItemID && !comment.DeletedAt.Valid {
			count++
		}
	}
	return count, nil
}

func (m *mockRepository) CreateActivityLog(ctx context.Context, params comments.CreateActivityLogParams) (comments.SprintManagementActivityLog, error) {
	log := comments.SprintManagementActivityLog{
		ID:         uuidToPgtype(uuid.New()),
		EntityType: params.EntityType,
		EntityID:   params.EntityID,
		Action:     params.Action,
		UserID:     params.UserID,
		Changes:    params.Changes,
		TraceID:    params.TraceID,
		CreatedAt:  pgtype.Timestamptz{Time: time.Now(), Valid: true},
	}
	m.activityLogs = append(m.activityLogs, log)
	return log, nil
}

func (m *mockRepository) ListActivityLogsByEntity(ctx context.Context, params comments.ListActivityLogsByEntityParams) ([]comments.SprintManagementActivityLog, error) {
	result := make([]comments.SprintManagementActivityLog, 0)
	for _, log := range m.activityLogs {
		if log.EntityType == params.EntityType && log.EntityID == params.EntityID {
			result = append(result, log)
		}
	}
	return result, nil
}

func (m *mockRepository) ListActivityLogsByUser(ctx context.Context, params comments.ListActivityLogsByUserParams) ([]comments.SprintManagementActivityLog, error) {
	result := make([]comments.SprintManagementActivityLog, 0)
	for _, log := range m.activityLogs {
		if log.UserID == params.UserID {
			result = append(result, log)
		}
	}
	return result, nil
}

func (m *mockRepository) ListRecentActivityLogs(ctx context.Context, params comments.ListRecentActivityLogsParams) ([]comments.SprintManagementActivityLog, error) {
	return m.activityLogs, nil
}

// Feature: go-sprint-management, Property: Comment Lifecycle and Soft Delete
// Validates: Requirements 32.1, 32.2, 37.1
func TestProperty_CommentLifecycleAndSoftDelete(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("created comments can be retrieved", prop.ForAll(
		func(content string) bool {
			if len(content) == 0 {
				return true // Skip empty content
			}

			repo := newMockRepository()
			service := NewService(repo, newTestLogger())
			ctx := context.Background()

			workItemID := uuid.New()
			authorID := uuid.New()

			req := CreateCommentRequest{
				WorkItemID: workItemID,
				AuthorID:   authorID,
				Content:    content,
			}

			// Create comment
			comment, err := service.CreateComment(ctx, req)
			if err != nil {
				return false
			}

			// Retrieve comment
			retrieved, err := service.GetComment(ctx, comment.ID)
			if err != nil {
				return false
			}

			// Verify content matches
			return retrieved.Content == content &&
				retrieved.WorkItemID == workItemID &&
				retrieved.AuthorID == authorID
		},
		gen.AnyString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 1000 }),
	))

	properties.Property("soft deleted comments are not retrievable", prop.ForAll(
		func() bool {
			repo := newMockRepository()
			service := NewService(repo, newTestLogger())
			ctx := context.Background()

			workItemID := uuid.New()
			authorID := uuid.New()

			req := CreateCommentRequest{
				WorkItemID: workItemID,
				AuthorID:   authorID,
				Content:    "Test comment",
			}

			// Create comment
			comment, err := service.CreateComment(ctx, req)
			if err != nil {
				return false
			}

			// Delete comment
			err = service.DeleteComment(ctx, comment.ID, authorID)
			if err != nil {
				return false
			}

			// Try to retrieve deleted comment
			_, err = service.GetComment(ctx, comment.ID)
			return err == ErrCommentNotFound
		},
	))

	properties.Property("only author can update comment", prop.ForAll(
		func() bool {
			repo := newMockRepository()
			service := NewService(repo, newTestLogger())
			ctx := context.Background()

			workItemID := uuid.New()
			authorID := uuid.New()
			otherUserID := uuid.New()

			req := CreateCommentRequest{
				WorkItemID: workItemID,
				AuthorID:   authorID,
				Content:    "Original content",
			}

			// Create comment
			comment, err := service.CreateComment(ctx, req)
			if err != nil {
				return false
			}

			// Try to update as different user
			updateReq := UpdateCommentRequest{
				Content: "Updated content",
			}
			_, err = service.UpdateComment(ctx, comment.ID, updateReq, otherUserID)
			unauthorizedFails := err == ErrUnauthorized

			// Update as author should succeed
			_, err = service.UpdateComment(ctx, comment.ID, updateReq, authorID)
			authorSucceeds := err == nil

			return unauthorizedFails && authorSucceeds
		},
	))

	properties.Property("only author can delete comment", prop.ForAll(
		func() bool {
			repo := newMockRepository()
			service := NewService(repo, newTestLogger())
			ctx := context.Background()

			workItemID := uuid.New()
			authorID := uuid.New()
			otherUserID := uuid.New()

			req := CreateCommentRequest{
				WorkItemID: workItemID,
				AuthorID:   authorID,
				Content:    "Test comment",
			}

			// Create comment
			comment, err := service.CreateComment(ctx, req)
			if err != nil {
				return false
			}

			// Try to delete as different user
			err = service.DeleteComment(ctx, comment.ID, otherUserID)
			unauthorizedFails := err == ErrUnauthorized

			// Delete as author should succeed
			err = service.DeleteComment(ctx, comment.ID, authorID)
			authorSucceeds := err == nil

			return unauthorizedFails && authorSucceeds
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property: Activity Log Completeness and Accuracy
// Validates: Requirements 21.1, 21.2, 21.3, 21.4, 32.2
func TestProperty_ActivityLogCompletenessAndAccuracy(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("comment creation logs activity", prop.ForAll(
		func(content string) bool {
			if len(content) == 0 {
				return true // Skip empty content
			}

			repo := newMockRepository()
			service := NewService(repo, newTestLogger())
			ctx := context.Background()

			workItemID := uuid.New()
			authorID := uuid.New()

			req := CreateCommentRequest{
				WorkItemID: workItemID,
				AuthorID:   authorID,
				Content:    content,
			}

			// Create comment
			comment, err := service.CreateComment(ctx, req)
			if err != nil {
				return false
			}

			// Check activity log was created
			logs, err := service.GetActivityFeed(ctx, ActivityFeedRequest{
				EntityType: "comment",
				EntityID:   &comment.ID,
				Limit:      10,
				Offset:     0,
			})
			if err != nil {
				return false
			}

			// Should have at least one log entry
			if len(logs) == 0 {
				return false
			}

			// Verify log details
			log := logs[0]
			return log.EntityType == "comment" &&
				log.EntityID == comment.ID &&
				log.Action == "commented" &&
				log.UserID == authorID
		},
		gen.AnyString().SuchThat(func(s string) bool { return len(s) > 0 && len(s) < 1000 }),
	))

	properties.Property("comment update logs activity with changes", prop.ForAll(
		func() bool {
			repo := newMockRepository()
			service := NewService(repo, newTestLogger())
			ctx := context.Background()

			workItemID := uuid.New()
			authorID := uuid.New()

			req := CreateCommentRequest{
				WorkItemID: workItemID,
				AuthorID:   authorID,
				Content:    "Original content",
			}

			// Create comment
			comment, err := service.CreateComment(ctx, req)
			if err != nil {
				return false
			}

			// Update comment
			updateReq := UpdateCommentRequest{
				Content: "Updated content",
			}
			_, err = service.UpdateComment(ctx, comment.ID, updateReq, authorID)
			if err != nil {
				return false
			}

			// Check activity log
			logs, err := service.GetActivityFeed(ctx, ActivityFeedRequest{
				EntityType: "comment",
				EntityID:   &comment.ID,
				Limit:      10,
				Offset:     0,
			})
			if err != nil {
				return false
			}

			// Should have at least 2 log entries (create + update)
			if len(logs) < 2 {
				return false
			}

			// Find the update log
			var updateLog *comments.SprintManagementActivityLog
			for _, log := range repo.activityLogs {
				if log.Action == "updated" {
					updateLog = &log
					break
				}
			}

			if updateLog == nil {
				return false
			}

			// Verify changes are logged
			var changes map[string]interface{}
			if err := json.Unmarshal(updateLog.Changes, &changes); err != nil {
				return false
			}

			return changes["before"] == "Original content" &&
				changes["after"] == "Updated content"
		},
	))

	properties.Property("comment deletion logs activity", prop.ForAll(
		func() bool {
			repo := newMockRepository()
			service := NewService(repo, newTestLogger())
			ctx := context.Background()

			workItemID := uuid.New()
			authorID := uuid.New()

			req := CreateCommentRequest{
				WorkItemID: workItemID,
				AuthorID:   authorID,
				Content:    "Test comment",
			}

			// Create comment
			comment, err := service.CreateComment(ctx, req)
			if err != nil {
				return false
			}

			// Delete comment
			err = service.DeleteComment(ctx, comment.ID, authorID)
			if err != nil {
				return false
			}

			// Check activity log
			logs, err := service.GetActivityFeed(ctx, ActivityFeedRequest{
				EntityType: "comment",
				EntityID:   &comment.ID,
				Limit:      10,
				Offset:     0,
			})
			if err != nil {
				return false
			}

			// Should have at least 2 log entries (create + delete)
			if len(logs) < 2 {
				return false
			}

			// Find the delete log
			var deleteLog *comments.SprintManagementActivityLog
			for _, log := range repo.activityLogs {
				if log.Action == "deleted" {
					deleteLog = &log
					break
				}
			}

			return deleteLog != nil &&
				deleteLog.EntityType == "comment" &&
				pgtypeToUUID(deleteLog.EntityID) == comment.ID
		},
	))

	properties.Property("activity logs include user identification", prop.ForAll(
		func() bool {
			repo := newMockRepository()
			service := NewService(repo, newTestLogger())
			ctx := context.Background()

			workItemID := uuid.New()
			authorID := uuid.New()

			req := CreateCommentRequest{
				WorkItemID: workItemID,
				AuthorID:   authorID,
				Content:    "Test comment",
			}

			// Create comment
			_, err := service.CreateComment(ctx, req)
			if err != nil {
				return false
			}

			// Get activity logs for user
			logs, err := service.GetActivityFeed(ctx, ActivityFeedRequest{
				UserID: &authorID,
				Limit:  10,
				Offset: 0,
			})
			if err != nil {
				return false
			}

			// Should have at least one log entry
			if len(logs) == 0 {
				return false
			}

			// All logs should have the correct user ID
			for _, log := range logs {
				if log.UserID != authorID {
					return false
				}
			}

			return true
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// Feature: go-sprint-management, Property: Comment Threading
// Validates: Requirements 32.3, 32.4
func TestProperty_CommentThreading(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("all comments for work item are retrievable", prop.ForAll(
		func(numComments uint8) bool {
			if numComments == 0 || numComments > 20 {
				return true // Skip edge cases
			}

			repo := newMockRepository()
			service := NewService(repo, newTestLogger())
			ctx := context.Background()

			workItemID := uuid.New()
			authorID := uuid.New()

			// Create multiple comments
			for i := uint8(0); i < numComments; i++ {
				req := CreateCommentRequest{
					WorkItemID: workItemID,
					AuthorID:   authorID,
					Content:    "Comment " + string(rune(i)),
				}
				_, err := service.CreateComment(ctx, req)
				if err != nil {
					return false
				}
			}

			// Retrieve all comments for work item
			commentList, err := service.ListCommentsByWorkItem(ctx, workItemID)
			if err != nil {
				return false
			}

			return len(commentList) == int(numComments)
		},
		gen.UInt8(),
	))

	properties.Property("soft deleted comments excluded from work item list", prop.ForAll(
		func() bool {
			repo := newMockRepository()
			service := NewService(repo, newTestLogger())
			ctx := context.Background()

			workItemID := uuid.New()
			authorID := uuid.New()

			// Create 3 comments
			var commentIDs []uuid.UUID
			for i := 0; i < 3; i++ {
				req := CreateCommentRequest{
					WorkItemID: workItemID,
					AuthorID:   authorID,
					Content:    "Comment",
				}
				comment, err := service.CreateComment(ctx, req)
				if err != nil {
					return false
				}
				commentIDs = append(commentIDs, comment.ID)
			}

			// Delete one comment
			err := service.DeleteComment(ctx, commentIDs[1], authorID)
			if err != nil {
				return false
			}

			// Retrieve comments
			commentList, err := service.ListCommentsByWorkItem(ctx, workItemID)
			if err != nil {
				return false
			}

			// Should only have 2 comments (3 created - 1 deleted)
			return len(commentList) == 2
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}
