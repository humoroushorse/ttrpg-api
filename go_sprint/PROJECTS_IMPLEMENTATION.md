# Projects Feature Implementation

## Status: 95% Complete

This document tracks the implementation of the Projects feature for organizing work items.

## Completed ✅
### Backend (Go) - 100%
- ✅ Database migrations (000003_create_projects_table)
- ✅ Project model (internal/models/project.go)
- ✅ Project repository (internal/repository/project_repository.go)
- ✅ Project handlers (internal/handlers/projects.go)
- ✅ Project routes added to main.go
- ✅ Auto-incrementing ticket numbers with trigger

### Frontend (Angular) - 95%
- ✅ Project models (features/sprint-management/models/src/lib/project.models.ts)
- ✅ Project API Service (features/sprint-management/data-access/src/lib/service/project-api.service.ts)
- ✅ Project Store (features/sprint-management/data-access/src/lib/+state/project.store.ts)
- ✅ Project List Page (features/sprint-management/feature-shell/src/lib/pages/projects/page-projects-list.component.*)
- ✅ Project Create Page (features/sprint-management/feature-shell/src/lib/pages/projects/page-project-create.component.*)
- ✅ Project Edit Page (features/sprint-management/feature-shell/src/lib/pages/projects/page-project-edit.component.*)
- ✅ Project Detail Page (features/sprint-management/feature-shell/src/lib/pages/projects/page-project-detail.component.*)
- ✅ Added project routes to lib.routes.ts
- ✅ Added "Projects" to sidenav.routes.ts
- ✅ Updated Work Item model to include project_id and ticket_number
- ✅ Updated Work Item forms to include project selector
- ✅ Exported from libraries

## Remaining Work 🚧
### Frontend (Angular) - NEXT STEPS
1. Update displays to show ticket numbers (DND-34) instead of UUIDs:
   - Work item list page
   - Work item detail page
   - Sprint board component
   - Sprint detail page (work items tab)
2. Add helper function to format ticket display (project.key + "-" + ticket_number)
3. Update table columns to show ticket numbers

### E2E Tests
1. Project CRUD tests
2. Ticket number generation tests
3. Work item with project tests

## Quick Start

### 1. Apply Migrations
```bash
cd ttrpg-api/go_sprint
make migrate-up
make stop
make run
```

### 2. Test Backend API
```bash
# Create a project
curl -X POST http://localhost:8003/api/v1/projects \
  -H "Content-Type: application/json" \
  -d '{"key":"DND","name":"D&D Campaign","starting_number":34}'

# List projects
curl http://localhost:8003/api/v1/projects
```

### 3. Access UI
Navigate to: http://localhost:4204/sprint-management/projects

## Files Created/Updated

### Backend
- migrations/000003_create_projects_table.{up,down}.sql
- internal/models/project.go
- internal/repository/project_repository.go
- internal/handlers/projects.go
- cmd/server/main.go (updated)

### Frontend
- features/sprint-management/models/src/lib/project.models.ts
- features/sprint-management/models/src/lib/work-item.models.ts (updated)
- features/sprint-management/data-access/src/lib/service/project-api.service.ts
- features/sprint-management/data-access/src/lib/+state/project.store.ts
- features/sprint-management/feature-shell/src/lib/pages/projects/page-projects-list.component.*
- features/sprint-management/feature-shell/src/lib/pages/projects/page-project-create.component.*
- features/sprint-management/feature-shell/src/lib/pages/projects/page-project-edit.component.*
- features/sprint-management/feature-shell/src/lib/pages/projects/page-project-detail.component.*
- features/sprint-management/feature-shell/src/lib/pages/work-items/page-work-item-create.component.* (updated)
- features/sprint-management/feature-shell/src/lib/pages/work-items/page-work-item-edit.component.* (updated)
- features/sprint-management/feature-shell/src/lib/lib.routes.ts (updated)
- features/sprint-management/feature-shell/src/lib/sidenav.routes.ts (updated)

## Next Session TODO
- Update displays to show ticket numbers instead of UUIDs
- Add e2e tests


