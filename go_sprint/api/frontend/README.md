# Frontend Integration Assets

This directory contains TypeScript interfaces and Angular service examples for integrating the Sprint Management API with frontend applications.

## Contents

- **api-types.ts** - Complete TypeScript interface definitions generated from OpenAPI specifications
- **angular-services.example.ts** - Angular service examples for API integration
- **websocket.example.ts** - WebSocket service for real-time updates
- **component.example.ts** - Angular component examples demonstrating usage

## Quick Start

### 1. Copy TypeScript Interfaces

Copy the `api-types.ts` file to your Angular project:

```bash
cp api-types.ts your-angular-project/src/app/models/
```

### 2. Create API Services

Use the examples in `angular-services.example.ts` to create your own services:

```bash
# Create services directory
mkdir -p your-angular-project/src/app/services

# Copy and adapt the service examples
cp angular-services.example.ts your-angular-project/src/app/services/
```

### 3. Set Up WebSocket Service

Copy the WebSocket service for real-time updates:

```bash
cp websocket.example.ts your-angular-project/src/app/services/
```

### 4. Configure Environment

Update your environment files with API URLs:

```typescript
// src/environments/environment.ts
export const environment = {
  production: false,
  apiUrl: 'http://localhost:8082',
  wsUrl: 'ws://localhost:8082/ws',
  keycloakUrl: 'http://localhost:8080',
  keycloakRealm: 'sprint-management',
  keycloakClientId: 'sprint-ui'
};
```

## TypeScript Interfaces

The `api-types.ts` file contains:

### Core Models
- `WorkItem` - Work item model (epic, story, defect)
- `Sprint` - Sprint model
- `WorkItemDependency` - Dependency between work items
- `Comment` - Comment on work items
- `ActivityLog` - Activity history

### Request/Response Types
- `CreateWorkItemRequest` - Create work item payload
- `UpdateWorkItemRequest` - Update work item payload
- `CreateSprintRequest` - Create sprint payload
- `WorkItemListResponse` - Paginated work item list
- `SprintListResponse` - Paginated sprint list

### Metrics and Analytics
- `SprintMetrics` - Sprint progress metrics
- `BurndownData` - Burndown chart data
- `VelocityData` - Sprint velocity data
- `TeamVelocityHistory` - Historical velocity

### WebSocket Types
- `WebSocketMessage` - WebSocket message format
- `WorkItemEventData` - Work item event payload
- `SprintEventData` - Sprint event payload

### Enums
- `WorkItemType` - epic, story, defect
- `WorkItemStatus` - todo, in_progress, in_review, done, blocked
- `PriorityLevel` - low, medium, high, critical
- `SprintStatus` - planned, active, completed, cancelled
- `DependencyType` - blocks, is_blocked_by, relates_to, duplicates

## Angular Services

### WorkItemsService

Provides methods for work item management:

```typescript
// List work items
listWorkItems(params?: WorkItemListParams): Observable<WorkItemListResponse>

// Get work item details
getWorkItem(id: string): Observable<WorkItemDetail>

// Create work item
createWorkItem(request: CreateWorkItemRequest): Observable<WorkItem>

// Update work item
updateWorkItem(id: string, request: UpdateWorkItemRequest): Observable<WorkItem>

// Delete work item
deleteWorkItem(id: string): Observable<void>

// Search work items
searchWorkItems(request: SearchWorkItemsRequest): Observable<WorkItemListResponse>

// Manage dependencies
getDependencies(id: string): Observable<DependencyListResponse>
createDependency(id: string, request: CreateDependencyRequest): Observable<WorkItemDependency>
deleteDependency(id: string, dependencyId: string): Observable<void>
```

### SprintsService

Provides methods for sprint management:

```typescript
// List sprints
listSprints(params?: SprintListParams): Observable<SprintListResponse>

// Get sprint details
getSprint(id: string): Observable<SprintDetail>

// Create sprint
createSprint(request: CreateSprintRequest): Observable<Sprint>

// Update sprint
updateSprint(id: string, request: UpdateSprintRequest): Observable<Sprint>

// Close sprint
closeSprint(id: string): Observable<any>

// Get metrics
getSprintMetrics(id: string): Observable<SprintMetrics>
getSprintBurndown(id: string): Observable<BurndownData>
getSprintVelocity(id: string): Observable<VelocityData>

// Planning
getTeamVelocityHistory(): Observable<TeamVelocityHistory>
getSprintForecast(): Observable<SprintForecast>
```

### WebSocketService

Provides real-time updates:

```typescript
// Connection management
connect(token: string): void
disconnect(): void
isConnected(): boolean
getState(): Observable<WebSocketState>

// Room management
joinRoom(room: string): void
leaveRoom(room: string): void

// Event subscriptions
onWorkItemCreated(): Observable<WorkItemEventData>
onWorkItemUpdated(): Observable<WorkItemEventData>
onWorkItemStatusChanged(): Observable<WorkItemEventData>
onSprintCreated(): Observable<SprintEventData>
onSprintUpdated(): Observable<SprintEventData>
onSprintClosed(): Observable<SprintEventData>
onCommentAdded(): Observable<CommentEventData>
```

## Usage Examples

### Creating a Work Item

```typescript
import { WorkItemsService } from './services/work-items.service';
import { WorkItemType, PriorityLevel } from './models/api-types';

constructor(private workItemsService: WorkItemsService) {}

createWorkItem() {
  const request = {
    type: WorkItemType.Story,
    title: 'User login feature',
    description: 'Implement user authentication',
    priority: PriorityLevel.High,
    story_points: 5
  };
  
  this.workItemsService.createWorkItem(request).subscribe({
    next: (workItem) => console.log('Created:', workItem),
    error: (error) => console.error('Error:', error)
  });
}
```

### Real-time Updates

```typescript
import { WebSocketService } from './services/websocket.service';

constructor(private wsService: WebSocketService) {}

ngOnInit() {
  // Connect to WebSocket
  const token = this.authService.getToken();
  this.wsService.connect(token);
  
  // Join sprint room
  this.wsService.joinRoom('sprint:sprint-123');
  
  // Subscribe to updates
  this.wsService.onWorkItemUpdated().subscribe(event => {
    console.log('Work item updated:', event.work_item);
    // Update UI
  });
}

ngOnDestroy() {
  this.wsService.leaveRoom('sprint:sprint-123');
  this.wsService.disconnect();
}
```

### Displaying Sprint Burndown

```typescript
import { SprintsService } from './services/sprints.service';

constructor(private sprintsService: SprintsService) {}

loadBurndown(sprintId: string) {
  this.sprintsService.getSprintBurndown(sprintId).subscribe({
    next: (data) => {
      // Transform data for your chart library
      this.chartData = {
        actual: data.data_points.map(p => ({
          date: p.date,
          points: p.remaining_points
        })),
        ideal: data.ideal_line.map(p => ({
          date: p.date,
          points: p.ideal_points
        }))
      };
    }
  });
}
```

## Authentication

All API requests require JWT authentication. Set up an HTTP interceptor:

```typescript
import { Injectable } from '@angular/core';
import { HttpInterceptor, HttpRequest, HttpHandler } from '@angular/common/http';

@Injectable()
export class AuthInterceptor implements HttpInterceptor {
  intercept(req: HttpRequest<any>, next: HttpHandler) {
    const token = localStorage.getItem('jwt_token');
    
    if (token) {
      req = req.clone({
        setHeaders: {
          Authorization: `Bearer ${token}`
        }
      });
    }
    
    return next.handle(req);
  }
}
```

Register in your app module:

```typescript
import { HTTP_INTERCEPTORS } from '@angular/common/http';

@NgModule({
  providers: [
    {
      provide: HTTP_INTERCEPTORS,
      useClass: AuthInterceptor,
      multi: true
    }
  ]
})
export class AppModule {}
```

## Error Handling

All API errors follow a consistent format:

```typescript
interface ErrorResponse {
  error: {
    code: string;
    message: string;
    details?: any;
    validation_errors?: ValidationError[];
  };
  trace_id: string;
  timestamp: string;
}
```

Handle errors in your services:

```typescript
private handleError(error: any): Observable<never> {
  if (error.error && error.error.error) {
    const errorResponse: ErrorResponse = error.error;
    console.error('API Error:', {
      code: errorResponse.error.code,
      message: errorResponse.error.message,
      trace_id: errorResponse.trace_id
    });
  }
  return throwError(() => error);
}
```

## Pagination

All list endpoints use cursor-based pagination:

```typescript
// Initial request
this.workItemsService.listWorkItems({ limit: 20 }).subscribe(response => {
  this.workItems = response.items;
  this.nextCursor = response.pagination.next_cursor;
  this.hasMore = response.pagination.has_more;
});

// Load next page
if (this.nextCursor) {
  this.workItemsService.listWorkItems({
    cursor: this.nextCursor,
    limit: 20
  }).subscribe(response => {
    this.workItems = [...this.workItems, ...response.items];
    this.nextCursor = response.pagination.next_cursor;
    this.hasMore = response.pagination.has_more;
  });
}
```

## Additional Resources

- [Frontend Integration Guide](../../docs/frontend-integration.md) - Comprehensive integration guide
- [API Documentation](../../docs/api/api-documentation.html) - Full API reference
- [OpenAPI Specification](../openapi/main.yaml) - API specification
- [WebSocket Events](../../docs/frontend-integration.md#websocket-integration) - Real-time event documentation

## Support

For issues or questions:
- Check the [Frontend Integration Guide](../../docs/frontend-integration.md)
- Review the [API Documentation](../../docs/api/api-documentation.html)
- Contact the development team

## License

MIT License - See LICENSE file for details
