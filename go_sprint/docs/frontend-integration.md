# Frontend Integration Guide

This guide provides comprehensive instructions for integrating the Sprint Management API with Angular frontend applications.

## Table of Contents

- [Overview](#overview)
- [Authentication](#authentication)
- [TypeScript Interfaces](#typescript-interfaces)
- [Angular HTTP Client Service](#angular-http-client-service)
- [WebSocket Integration](#websocket-integration)
- [Error Handling](#error-handling)
- [Example Usage](#example-usage)

## Overview

The Sprint Management API provides RESTful endpoints for managing work items, sprints, and team collaboration. All endpoints require JWT authentication via Keycloak.

### Base URL

- **Local Development**: `http://localhost:8082`
- **Production**: `https://api.sprint.example.com`

### API Version

Current API version: `v1`

All endpoints are prefixed with `/api/v1`

## Authentication

### JWT Token Authentication

All API requests require a valid JWT token from Keycloak in the Authorization header:

```typescript
Authorization: Bearer <jwt_token>
```

### Keycloak Integration Flow

The Sprint Management API uses Keycloak for authentication. Here's the complete authentication flow:

1. **User Login**: User authenticates with Keycloak
2. **Token Retrieval**: Keycloak returns JWT access token and refresh token
3. **API Requests**: Include JWT token in Authorization header
4. **Token Validation**: Sprint Management Service validates token via Auth Service
5. **Token Refresh**: Use refresh token to get new access token when expired

### Keycloak Angular Setup

Install the Keycloak Angular adapter:

```bash
npm install keycloak-angular keycloak-js
```

Configure Keycloak in your app:

```typescript
// app.module.ts
import { APP_INITIALIZER, NgModule } from '@angular/core';
import { KeycloakAngularModule, KeycloakService } from 'keycloak-angular';

function initializeKeycloak(keycloak: KeycloakService) {
  return () =>
    keycloak.init({
      config: {
        url: 'http://localhost:8080',
        realm: 'sprint-management',
        clientId: 'sprint-ui'
      },
      initOptions: {
        onLoad: 'check-sso',
        silentCheckSsoRedirectUri:
          window.location.origin + '/assets/silent-check-sso.html'
      }
    });
}

@NgModule({
  imports: [KeycloakAngularModule],
  providers: [
    {
      provide: APP_INITIALIZER,
      useFactory: initializeKeycloak,
      multi: true,
      deps: [KeycloakService]
    }
  ]
})
export class AppModule {}
```

### Auth Service

Create an authentication service to manage tokens:

```typescript
// auth.service.ts
import { Injectable } from '@angular/core';
import { KeycloakService } from 'keycloak-angular';
import { KeycloakProfile } from 'keycloak-js';
import { from, Observable } from 'rxjs';

@Injectable({
  providedIn: 'root'
})
export class AuthService {
  constructor(private keycloakService: KeycloakService) {}

  /**
   * Get current JWT access token
   */
  getToken(): string {
    return this.keycloakService.getKeycloakInstance().token || '';
  }

  /**
   * Get user profile
   */
  getUserProfile(): Observable<KeycloakProfile> {
    return from(this.keycloakService.loadUserProfile());
  }

  /**
   * Check if user is logged in
   */
  isLoggedIn(): boolean {
    return this.keycloakService.isLoggedIn();
  }

  /**
   * Get user roles
   */
  getUserRoles(): string[] {
    return this.keycloakService.getUserRoles();
  }

  /**
   * Login user
   */
  login(): void {
    this.keycloakService.login();
  }

  /**
   * Logout user
   */
  logout(): void {
    this.keycloakService.logout();
  }

  /**
   * Check if user has specific role
   */
  hasRole(role: string): boolean {
    return this.keycloakService.isUserInRole(role);
  }
}
```

### Angular HTTP Interceptor

Create an HTTP interceptor to automatically add the JWT token to all requests:

```typescript
// auth.interceptor.ts
import { Injectable } from '@angular/core';
import { HttpInterceptor, HttpRequest, HttpHandler, HttpEvent } from '@angular/common/http';
import { Observable } from 'rxjs';
import { AuthService } from './auth.service';

@Injectable()
export class AuthInterceptor implements HttpInterceptor {
  constructor(private authService: AuthService) {}

  intercept(req: HttpRequest<any>, next: HttpHandler): Observable<HttpEvent<any>> {
    const token = this.authService.getToken();
    
    if (token) {
      const cloned = req.clone({
        headers: req.headers.set('Authorization', `Bearer ${token}`)
      });
      return next.handle(cloned);
    }
    
    return next.handle(req);
  }
}
```

### Trace ID Correlation

Add trace IDs to requests for debugging and correlation:

```typescript
// trace.interceptor.ts
import { Injectable } from '@angular/core';
import { HttpInterceptor, HttpRequest, HttpHandler, HttpEvent } from '@angular/common/http';
import { Observable } from 'rxjs';
import { v4 as uuidv4 } from 'uuid';

@Injectable()
export class TraceInterceptor implements HttpInterceptor {
  intercept(req: HttpRequest<any>, next: HttpHandler): Observable<HttpEvent<any>> {
    const traceId = uuidv4();
    
    const cloned = req.clone({
      headers: req.headers.set('X-Trace-ID', traceId)
    });
    
    // Store trace ID for error reporting
    (cloned as any).traceId = traceId;
    
    return next.handle(cloned);
  }
}
```

### Route Guards

Protect routes that require authentication:

```typescript
// auth.guard.ts
import { Injectable } from '@angular/core';
import { ActivatedRouteSnapshot, Router, RouterStateSnapshot, UrlTree } from '@angular/router';
import { KeycloakAuthGuard, KeycloakService } from 'keycloak-angular';

@Injectable({
  providedIn: 'root'
})
export class AuthGuard extends KeycloakAuthGuard {
  constructor(
    protected override readonly router: Router,
    protected readonly keycloak: KeycloakService
  ) {
    super(router, keycloak);
  }

  async isAccessAllowed(
    route: ActivatedRouteSnapshot,
    state: RouterStateSnapshot
  ): Promise<boolean | UrlTree> {
    // Force the user to log in if not authenticated
    if (!this.authenticated) {
      await this.keycloak.login({
        redirectUri: window.location.origin + state.url
      });
    }

    // Get required roles from route data
    const requiredRoles = route.data['roles'];

    // Allow if no roles are required
    if (!requiredRoles || requiredRoles.length === 0) {
      return true;
    }

    // Check if user has required roles
    return requiredRoles.some((role: string) => this.roles.includes(role));
  }
}
```

Use the guard in your routes:

```typescript
// app-routing.module.ts
import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { AuthGuard } from './guards/auth.guard';

const routes: Routes = [
  {
    path: 'sprints',
    component: SprintListComponent,
    canActivate: [AuthGuard]
  },
  {
    path: 'sprints/:id',
    component: SprintBoardComponent,
    canActivate: [AuthGuard],
    data: { roles: ['user'] }
  },
  {
    path: 'admin',
    component: AdminComponent,
    canActivate: [AuthGuard],
    data: { roles: ['admin'] }
  }
];

@NgModule({
  imports: [RouterModule.forRoot(routes)],
  exports: [RouterModule]
})
export class AppRoutingModule {}
```

### Registering Interceptors

Register all interceptors in your app module:

```typescript
// app.module.ts
import { NgModule } from '@angular/core';
import { HTTP_INTERCEPTORS, HttpClientModule } from '@angular/common/http';
import { AuthInterceptor } from './interceptors/auth.interceptor';
import { TraceInterceptor } from './interceptors/trace.interceptor';
import { ErrorInterceptor } from './interceptors/error.interceptor';

@NgModule({
  imports: [
    HttpClientModule,
    // ... other imports
  ],
  providers: [
    {
      provide: HTTP_INTERCEPTORS,
      useClass: AuthInterceptor,
      multi: true
    },
    {
      provide: HTTP_INTERCEPTORS,
      useClass: TraceInterceptor,
      multi: true
    },
    {
      provide: HTTP_INTERCEPTORS,
      useClass: ErrorInterceptor,
      multi: true
    }
  ]
})
export class AppModule {}
```

## TypeScript Interfaces

### Generating TypeScript Interfaces

TypeScript interfaces are automatically generated from OpenAPI specifications:

```bash
# Generate TypeScript interfaces
make openapi-generate

# Or manually with openapi-typescript
openapi-typescript api/openapi/main.yaml --output api/frontend/api-types.ts
```

### Pre-generated Interfaces

Pre-generated TypeScript interfaces are available in `api/frontend/api-types.ts`. This file includes:

- **Core Models**: WorkItem, Sprint, Comment, ActivityLog
- **Request/Response Types**: CreateWorkItemRequest, UpdateSprintRequest, etc.
- **Enums**: WorkItemType, WorkItemStatus, PriorityLevel, SprintStatus
- **Pagination Types**: PaginationInfo, WorkItemListResponse
- **Metrics Types**: SprintMetrics, BurndownData, VelocityData
- **WebSocket Types**: WebSocketMessage, WorkItemEventData
- **Error Types**: ErrorResponse, ValidationError
- **Type Guards**: isErrorResponse(), isEpic(), isActiveSprint()

### Using Generated Types

```typescript
// Import generated types
import { 
  WorkItem, 
  CreateWorkItemRequest, 
  Sprint,
  SprintMetrics,
  WorkItemType,
  PriorityLevel 
} from './api-types';

// Use in your components
export class WorkItemComponent {
  workItem: WorkItem;
  
  createWorkItem(request: CreateWorkItemRequest): void {
    // Type-safe API calls
  }
  
  // Use enums for type safety
  isHighPriority(workItem: WorkItem): boolean {
    return workItem.priority === PriorityLevel.High || 
           workItem.priority === PriorityLevel.Critical;
  }
}
```

### Core Type Definitions

```typescript
// work-item.types.ts
export enum WorkItemType {
  Epic = 'epic',
  Story = 'story',
  Defect = 'defect'
}

export enum WorkItemStatus {
  Todo = 'todo',
  InProgress = 'in_progress',
  InReview = 'in_review',
  Done = 'done',
  Blocked = 'blocked'
}

export enum PriorityLevel {
  Low = 'low',
  Medium = 'medium',
  High = 'high',
  Critical = 'critical'
}

export interface WorkItem {
  id: string;
  type: WorkItemType;
  title: string;
  description: string;
  status: WorkItemStatus;
  priority: PriorityLevel;
  story_points?: number;
  assignee_id?: string;
  reporter_id: string;
  parent_id?: string;
  sprint_id?: string;
  created_at: string;
  updated_at: string;
}

export interface CreateWorkItemRequest {
  type: WorkItemType;
  title: string;
  description: string;
  priority: PriorityLevel;
  story_points?: number;
  assignee_id?: string;
  parent_id?: string;
  sprint_id?: string;
}

export interface PaginationInfo {
  next_cursor?: string;
  has_more: boolean;
  total_count?: number;
}

export interface WorkItemListResponse {
  items: WorkItem[];
  pagination: PaginationInfo;
}
```

## Angular HTTP Client Service

### Work Items Service

```typescript
// work-items.service.ts
import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../environments/environment';
import { 
  WorkItem, 
  CreateWorkItemRequest, 
  UpdateWorkItemRequest,
  WorkItemListResponse,
  WorkItemDetail 
} from './api-types';

@Injectable({
  providedIn: 'root'
})
export class WorkItemsService {
  private readonly baseUrl = `${environment.apiUrl}/api/v1/workitems`;

  constructor(private http: HttpClient) {}

  /**
   * List work items with optional filtering and pagination
   */
  listWorkItems(filters?: {
    type?: string;
    status?: string;
    assignee_id?: string;
    sprint_id?: string;
    priority?: string;
    search?: string;
    cursor?: string;
    limit?: number;
  }): Observable<WorkItemListResponse> {
    let params = new HttpParams();
    
    if (filters) {
      Object.keys(filters).forEach(key => {
        const value = filters[key as keyof typeof filters];
        if (value !== undefined && value !== null) {
          params = params.set(key, value.toString());
        }
      });
    }
    
    return this.http.get<WorkItemListResponse>(this.baseUrl, { params });
  }

  /**
   * Get a specific work item by ID
   */
  getWorkItem(id: string): Observable<WorkItemDetail> {
    return this.http.get<WorkItemDetail>(`${this.baseUrl}/${id}`);
  }

  /**
   * Create a new work item
   */
  createWorkItem(request: CreateWorkItemRequest): Observable<WorkItem> {
    return this.http.post<WorkItem>(this.baseUrl, request);
  }

  /**
   * Update an existing work item
   */
  updateWorkItem(id: string, request: UpdateWorkItemRequest): Observable<WorkItem> {
    return this.http.put<WorkItem>(`${this.baseUrl}/${id}`, request);
  }

  /**
   * Delete a work item (soft delete)
   */
  deleteWorkItem(id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${id}`);
  }

  /**
   * Get child work items (sub-tasks)
   */
  getChildWorkItems(id: string): Observable<WorkItemListResponse> {
    return this.http.get<WorkItemListResponse>(`${this.baseUrl}/${id}/children`);
  }

  /**
   * Advanced search with multiple criteria
   */
  searchWorkItems(searchRequest: any): Observable<WorkItemListResponse> {
    return this.http.post<WorkItemListResponse>(`${this.baseUrl}/search`, searchRequest);
  }
}
```

### Sprints Service

```typescript
// sprints.service.ts
import { Injectable } from '@angular/core';
import { HttpClient, HttpParams } from '@angular/common/http';
import { Observable } from 'rxjs';
import { environment } from '../environments/environment';
import { 
  Sprint, 
  CreateSprintRequest, 
  UpdateSprintRequest,
  SprintListResponse,
  SprintDetail,
  SprintMetrics,
  BurndownData,
  VelocityData 
} from './api-types';

@Injectable({
  providedIn: 'root'
})
export class SprintsService {
  private readonly baseUrl = `${environment.apiUrl}/api/v1/sprints`;

  constructor(private http: HttpClient) {}

  /**
   * List sprints with optional filtering
   */
  listSprints(filters?: {
    status?: string;
    start_date_after?: string;
    start_date_before?: string;
    cursor?: string;
    limit?: number;
  }): Observable<SprintListResponse> {
    let params = new HttpParams();
    
    if (filters) {
      Object.keys(filters).forEach(key => {
        const value = filters[key as keyof typeof filters];
        if (value !== undefined && value !== null) {
          params = params.set(key, value.toString());
        }
      });
    }
    
    return this.http.get<SprintListResponse>(this.baseUrl, { params });
  }

  /**
   * Get a specific sprint by ID
   */
  getSprint(id: string): Observable<SprintDetail> {
    return this.http.get<SprintDetail>(`${this.baseUrl}/${id}`);
  }

  /**
   * Create a new sprint
   */
  createSprint(request: CreateSprintRequest): Observable<Sprint> {
    return this.http.post<Sprint>(this.baseUrl, request);
  }

  /**
   * Update an existing sprint
   */
  updateSprint(id: string, request: UpdateSprintRequest): Observable<Sprint> {
    return this.http.put<Sprint>(`${this.baseUrl}/${id}`, request);
  }

  /**
   * Close a sprint
   */
  closeSprint(id: string): Observable<any> {
    return this.http.post(`${this.baseUrl}/${id}/close`, {});
  }

  /**
   * Get sprint metrics
   */
  getSprintMetrics(id: string): Observable<SprintMetrics> {
    return this.http.get<SprintMetrics>(`${this.baseUrl}/${id}/metrics`);
  }

  /**
   * Get sprint burndown chart data
   */
  getSprintBurndown(id: string): Observable<BurndownData> {
    return this.http.get<BurndownData>(`${this.baseUrl}/${id}/burndown`);
  }

  /**
   * Get sprint velocity
   */
  getSprintVelocity(id: string): Observable<VelocityData> {
    return this.http.get<VelocityData>(`${this.baseUrl}/${id}/velocity`);
  }
}
```

## WebSocket Integration

### WebSocket Service

```typescript
// websocket.service.ts
import { Injectable } from '@angular/core';
import { Observable, Subject } from 'rxjs';
import { environment } from '../environments/environment';
import { AuthService } from './auth.service';

export interface WebSocketMessage {
  type: string;
  room?: string;
  data: any;
  trace_id: string;
  timestamp: string;
}

@Injectable({
  providedIn: 'root'
})
export class WebSocketService {
  private socket: WebSocket | null = null;
  private messageSubject = new Subject<WebSocketMessage>();
  
  constructor(private authService: AuthService) {}

  /**
   * Connect to WebSocket server
   */
  connect(): void {
    const token = this.authService.getToken();
    const wsUrl = `${environment.wsUrl}?token=${token}`;
    
    this.socket = new WebSocket(wsUrl);
    
    this.socket.onopen = () => {
      console.log('WebSocket connected');
    };
    
    this.socket.onmessage = (event) => {
      const message: WebSocketMessage = JSON.parse(event.data);
      this.messageSubject.next(message);
    };
    
    this.socket.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
    
    this.socket.onclose = () => {
      console.log('WebSocket disconnected');
      // Implement reconnection logic
      setTimeout(() => this.connect(), 5000);
    };
  }

  /**
   * Subscribe to WebSocket messages
   */
  messages(): Observable<WebSocketMessage> {
    return this.messageSubject.asObservable();
  }

  /**
   * Subscribe to a specific room (sprint, project)
   */
  joinRoom(room: string): void {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({
        type: 'join_room',
        room: room
      }));
    }
  }

  /**
   * Unsubscribe from a room
   */
  leaveRoom(room: string): void {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      this.socket.send(JSON.stringify({
        type: 'leave_room',
        room: room
      }));
    }
  }

  /**
   * Disconnect from WebSocket
   */
  disconnect(): void {
    if (this.socket) {
      this.socket.close();
      this.socket = null;
    }
  }
}
```

### Using WebSocket in Components

```typescript
// sprint-board.component.ts
import { Component, OnInit, OnDestroy } from '@angular/core';
import { WebSocketService, WebSocketMessage } from './websocket.service';
import { Subscription } from 'rxjs';
import { filter } from 'rxjs/operators';

@Component({
  selector: 'app-sprint-board',
  templateUrl: './sprint-board.component.html'
})
export class SprintBoardComponent implements OnInit, OnDestroy {
  private wsSubscription: Subscription;
  sprintId: string = 'sprint-123';

  constructor(private wsService: WebSocketService) {}

  ngOnInit(): void {
    // Connect to WebSocket
    this.wsService.connect();
    
    // Join sprint room
    this.wsService.joinRoom(`sprint:${this.sprintId}`);
    
    // Subscribe to messages
    this.wsSubscription = this.wsService.messages()
      .pipe(
        filter(msg => msg.room === `sprint:${this.sprintId}`)
      )
      .subscribe((message: WebSocketMessage) => {
        this.handleWebSocketMessage(message);
      });
  }

  ngOnDestroy(): void {
    // Leave room and disconnect
    this.wsService.leaveRoom(`sprint:${this.sprintId}`);
    if (this.wsSubscription) {
      this.wsSubscription.unsubscribe();
    }
  }

  private handleWebSocketMessage(message: WebSocketMessage): void {
    switch (message.type) {
      case 'workitem.created':
        console.log('New work item created:', message.data);
        // Update UI
        break;
      case 'workitem.updated':
        console.log('Work item updated:', message.data);
        // Update UI
        break;
      case 'sprint.status_changed':
        console.log('Sprint status changed:', message.data);
        // Update UI
        break;
    }
  }
}
```

## Error Handling

### Error Response Format

All API errors follow a consistent format:

```typescript
export interface ErrorResponse {
  error: ErrorDetail;
  trace_id: string;
  timestamp: string;
}

export interface ErrorDetail {
  code: string;
  message: string;
  details?: Record<string, any>;
  validation_errors?: ValidationError[];
}

export interface ValidationError {
  field: string;
  code: string;
  message: string;
}
```

### Error Interceptor

```typescript
// error.interceptor.ts
import { Injectable } from '@angular/core';
import { HttpInterceptor, HttpRequest, HttpHandler, HttpEvent, HttpErrorResponse } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError } from 'rxjs/operators';
import { ErrorResponse } from './api-types';

@Injectable()
export class ErrorInterceptor implements HttpInterceptor {
  intercept(req: HttpRequest<any>, next: HttpHandler): Observable<HttpEvent<any>> {
    return next.handle(req).pipe(
      catchError((error: HttpErrorResponse) => {
        if (error.error && error.error.error) {
          const errorResponse: ErrorResponse = error.error;
          
          // Log error with trace ID
          console.error('API Error:', {
            code: errorResponse.error.code,
            message: errorResponse.error.message,
            trace_id: errorResponse.trace_id,
            timestamp: errorResponse.timestamp
          });
          
          // Handle validation errors
          if (errorResponse.error.validation_errors) {
            console.error('Validation errors:', errorResponse.error.validation_errors);
          }
        }
        
        return throwError(() => error);
      })
    );
  }
}
```

## Example Usage

### Creating a Work Item

```typescript
// work-item-create.component.ts
import { Component } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { WorkItemsService } from './work-items.service';
import { WorkItemType, PriorityLevel } from './api-types';

@Component({
  selector: 'app-work-item-create',
  templateUrl: './work-item-create.component.html'
})
export class WorkItemCreateComponent {
  workItemForm: FormGroup;
  
  workItemTypes = Object.values(WorkItemType);
  priorities = Object.values(PriorityLevel);

  constructor(
    private fb: FormBuilder,
    private workItemsService: WorkItemsService
  ) {
    this.workItemForm = this.fb.group({
      type: [WorkItemType.Story, Validators.required],
      title: ['', [Validators.required, Validators.maxLength(255)]],
      description: ['', Validators.required],
      priority: [PriorityLevel.Medium, Validators.required],
      story_points: [null],
      assignee_id: [null],
      sprint_id: [null]
    });
  }

  onSubmit(): void {
    if (this.workItemForm.valid) {
      this.workItemsService.createWorkItem(this.workItemForm.value)
        .subscribe({
          next: (workItem) => {
            console.log('Work item created:', workItem);
            // Navigate or show success message
          },
          error: (error) => {
            console.error('Failed to create work item:', error);
            // Show error message
          }
        });
    }
  }
}
```

### Displaying Sprint Burndown Chart

```typescript
// sprint-burndown.component.ts
import { Component, OnInit, Input } from '@angular/core';
import { SprintsService } from './sprints.service';
import { BurndownData } from './api-types';

@Component({
  selector: 'app-sprint-burndown',
  templateUrl: './sprint-burndown.component.html'
})
export class SprintBurndownComponent implements OnInit {
  @Input() sprintId: string;
  
  burndownData: BurndownData;
  chartData: any;

  constructor(private sprintsService: SprintsService) {}

  ngOnInit(): void {
    this.loadBurndownData();
  }

  loadBurndownData(): void {
    this.sprintsService.getSprintBurndown(this.sprintId)
      .subscribe({
        next: (data) => {
          this.burndownData = data;
          this.prepareChartData();
        },
        error: (error) => {
          console.error('Failed to load burndown data:', error);
        }
      });
  }

  prepareChartData(): void {
    // Transform data for chart library (e.g., Chart.js, ngx-charts)
    this.chartData = {
      labels: this.burndownData.data_points.map(p => p.date),
      datasets: [
        {
          label: 'Actual',
          data: this.burndownData.data_points.map(p => p.remaining_points),
          borderColor: 'blue'
        },
        {
          label: 'Ideal',
          data: this.burndownData.ideal_line.map(p => p.remaining_points),
          borderColor: 'gray',
          borderDash: [5, 5]
        }
      ]
    };
  }
}
```

## Environment Configuration

```typescript
// environment.ts
export const environment = {
  production: false,
  apiUrl: 'http://localhost:8082',
  wsUrl: 'ws://localhost:8082/ws',
  keycloakUrl: 'http://localhost:8080',
  keycloakRealm: 'sprint-management',
  keycloakClientId: 'sprint-ui'
};

// environment.prod.ts
export const environment = {
  production: true,
  apiUrl: 'https://api.sprint.example.com',
  wsUrl: 'wss://api.sprint.example.com/ws',
  keycloakUrl: 'https://auth.example.com',
  keycloakRealm: 'sprint-management',
  keycloakClientId: 'sprint-ui'
};
```

## Next Steps

1. **Install Dependencies**: Add required npm packages to your Angular project
2. **Configure Interceptors**: Register HTTP interceptors in your app module
3. **Implement Services**: Create service classes for each API domain
4. **Build Components**: Create UI components using the services
5. **Test Integration**: Test API calls and WebSocket connections
6. **Handle Errors**: Implement comprehensive error handling
7. **Add Loading States**: Show loading indicators during API calls
8. **Implement Caching**: Cache frequently accessed data

## Complete Setup Guide

### Step 1: Install Dependencies

```bash
# Core dependencies
npm install keycloak-angular keycloak-js
npm install uuid
npm install @types/uuid --save-dev

# Optional: Chart libraries for burndown charts
npm install ngx-charts
npm install @swimlane/ngx-charts
```

### Step 2: Project Structure

Organize your Angular project:

```
src/app/
├── models/
│   └── api-types.ts              # Generated TypeScript interfaces
├── services/
│   ├── work-items.service.ts     # Work items API service
│   ├── sprints.service.ts        # Sprints API service
│   ├── comments.service.ts       # Comments API service
│   ├── websocket.service.ts      # WebSocket service
│   └── auth.service.ts           # Authentication service
├── interceptors/
│   ├── auth.interceptor.ts       # JWT token interceptor
│   ├── trace.interceptor.ts      # Trace ID interceptor
│   └── error.interceptor.ts      # Error handling interceptor
├── guards/
│   └── auth.guard.ts             # Route authentication guard
├── components/
│   ├── sprint-board/             # Sprint board component
│   ├── work-item-create/         # Work item creation form
│   ├── sprint-burndown/          # Burndown chart component
│   └── sprint-list/              # Sprint list component
└── environments/
    ├── environment.ts            # Development config
    └── environment.prod.ts       # Production config
```

### Step 3: Environment Configuration

Configure API endpoints:

```typescript
// src/environments/environment.ts
export const environment = {
  production: false,
  apiUrl: 'http://localhost:8082',
  wsUrl: 'ws://localhost:8082/ws',
  keycloak: {
    url: 'http://localhost:8080',
    realm: 'sprint-management',
    clientId: 'sprint-ui'
  }
};

// src/environments/environment.prod.ts
export const environment = {
  production: true,
  apiUrl: 'https://api.sprint.example.com',
  wsUrl: 'wss://api.sprint.example.com/ws',
  keycloak: {
    url: 'https://auth.example.com',
    realm: 'sprint-management',
    clientId: 'sprint-ui'
  }
};
```

### Step 4: Module Configuration

Configure your app module:

```typescript
// app.module.ts
import { NgModule, APP_INITIALIZER } from '@angular/core';
import { BrowserModule } from '@angular/platform-browser';
import { HttpClientModule, HTTP_INTERCEPTORS } from '@angular/common/http';
import { FormsModule, ReactiveFormsModule } from '@angular/forms';
import { KeycloakAngularModule, KeycloakService } from 'keycloak-angular';

import { AppRoutingModule } from './app-routing.module';
import { AppComponent } from './app.component';

// Services
import { WorkItemsService } from './services/work-items.service';
import { SprintsService } from './services/sprints.service';
import { WebSocketService } from './services/websocket.service';
import { AuthService } from './services/auth.service';

// Interceptors
import { AuthInterceptor } from './interceptors/auth.interceptor';
import { TraceInterceptor } from './interceptors/trace.interceptor';
import { ErrorInterceptor } from './interceptors/error.interceptor';

// Guards
import { AuthGuard } from './guards/auth.guard';

// Components
import { SprintBoardComponent } from './components/sprint-board/sprint-board.component';
import { WorkItemCreateComponent } from './components/work-item-create/work-item-create.component';
import { SprintBurndownComponent } from './components/sprint-burndown/sprint-burndown.component';
import { SprintListComponent } from './components/sprint-list/sprint-list.component';

function initializeKeycloak(keycloak: KeycloakService) {
  return () =>
    keycloak.init({
      config: {
        url: environment.keycloak.url,
        realm: environment.keycloak.realm,
        clientId: environment.keycloak.clientId
      },
      initOptions: {
        onLoad: 'check-sso',
        silentCheckSsoRedirectUri:
          window.location.origin + '/assets/silent-check-sso.html'
      }
    });
}

@NgModule({
  declarations: [
    AppComponent,
    SprintBoardComponent,
    WorkItemCreateComponent,
    SprintBurndownComponent,
    SprintListComponent
  ],
  imports: [
    BrowserModule,
    AppRoutingModule,
    HttpClientModule,
    FormsModule,
    ReactiveFormsModule,
    KeycloakAngularModule
  ],
  providers: [
    // Keycloak initialization
    {
      provide: APP_INITIALIZER,
      useFactory: initializeKeycloak,
      multi: true,
      deps: [KeycloakService]
    },
    // Services
    WorkItemsService,
    SprintsService,
    WebSocketService,
    AuthService,
    // Guards
    AuthGuard,
    // Interceptors
    {
      provide: HTTP_INTERCEPTORS,
      useClass: AuthInterceptor,
      multi: true
    },
    {
      provide: HTTP_INTERCEPTORS,
      useClass: TraceInterceptor,
      multi: true
    },
    {
      provide: HTTP_INTERCEPTORS,
      useClass: ErrorInterceptor,
      multi: true
    }
  ],
  bootstrap: [AppComponent]
})
export class AppModule {}
```

### Step 5: Routing Configuration

Set up protected routes:

```typescript
// app-routing.module.ts
import { NgModule } from '@angular/core';
import { RouterModule, Routes } from '@angular/router';
import { AuthGuard } from './guards/auth.guard';

import { SprintListComponent } from './components/sprint-list/sprint-list.component';
import { SprintBoardComponent } from './components/sprint-board/sprint-board.component';
import { WorkItemCreateComponent } from './components/work-item-create/work-item-create.component';

const routes: Routes = [
  { path: '', redirectTo: '/sprints', pathMatch: 'full' },
  {
    path: 'sprints',
    component: SprintListComponent,
    canActivate: [AuthGuard]
  },
  {
    path: 'sprints/:id',
    component: SprintBoardComponent,
    canActivate: [AuthGuard]
  },
  {
    path: 'work-items/create',
    component: WorkItemCreateComponent,
    canActivate: [AuthGuard]
  }
];

@NgModule({
  imports: [RouterModule.forRoot(routes)],
  exports: [RouterModule]
})
export class AppRoutingModule {}
```

### Step 6: Copy Generated Files

Copy the generated TypeScript interfaces and service examples:

```bash
# Copy TypeScript interfaces
cp go_sprint/api/frontend/api-types.ts your-angular-project/src/app/models/

# Copy service examples (adapt as needed)
cp go_sprint/api/frontend/angular-services.example.ts your-angular-project/src/app/services/
cp go_sprint/api/frontend/websocket.example.ts your-angular-project/src/app/services/
```

### Step 7: Testing

Test your integration:

```bash
# Start the Sprint Management API
cd go_sprint
make run

# Start your Angular app
cd your-angular-project
ng serve

# Open browser
open http://localhost:4203
```

## Best Practices

### 1. Error Handling

Always handle errors gracefully:

```typescript
this.workItemsService.createWorkItem(request).subscribe({
  next: (workItem) => {
    // Success handling
    this.showSuccessMessage('Work item created successfully');
  },
  error: (error) => {
    // Error handling
    if (error.error && error.error.error) {
      const errorResponse = error.error;
      this.showErrorMessage(errorResponse.error.message);
      
      // Log trace ID for debugging
      console.error('Trace ID:', errorResponse.trace_id);
    } else {
      this.showErrorMessage('An unexpected error occurred');
    }
  }
});
```

### 2. Loading States

Show loading indicators during API calls:

```typescript
export class WorkItemListComponent {
  isLoading = false;
  workItems: WorkItem[] = [];

  loadWorkItems(): void {
    this.isLoading = true;
    
    this.workItemsService.listWorkItems().subscribe({
      next: (response) => {
        this.workItems = response.items;
        this.isLoading = false;
      },
      error: (error) => {
        console.error('Error loading work items:', error);
        this.isLoading = false;
      }
    });
  }
}
```

### 3. Unsubscribe from Observables

Always unsubscribe to prevent memory leaks:

```typescript
export class SprintBoardComponent implements OnInit, OnDestroy {
  private subscriptions: Subscription[] = [];

  ngOnInit(): void {
    const sub = this.wsService.onWorkItemUpdated().subscribe(event => {
      // Handle update
    });
    this.subscriptions.push(sub);
  }

  ngOnDestroy(): void {
    this.subscriptions.forEach(sub => sub.unsubscribe());
  }
}
```

### 4. Type Safety

Leverage TypeScript for type safety:

```typescript
// Use enums instead of strings
const workItem: CreateWorkItemRequest = {
  type: WorkItemType.Story,  // Not 'story'
  priority: PriorityLevel.High,  // Not 'high'
  // ...
};

// Use type guards
if (isErrorResponse(response)) {
  console.error('Error:', response.error.message);
}
```

### 5. Caching

Implement caching for frequently accessed data:

```typescript
@Injectable({
  providedIn: 'root'
})
export class SprintsService {
  private cache = new Map<string, Sprint>();

  getSprint(id: string): Observable<Sprint> {
    // Check cache first
    if (this.cache.has(id)) {
      return of(this.cache.get(id)!);
    }
    
    // Fetch from API
    return this.http.get<Sprint>(`${this.baseUrl}/${id}`).pipe(
      tap(sprint => this.cache.set(id, sprint))
    );
  }
}
```

### 6. Retry Logic

Implement retry logic for transient failures:

```typescript
import { retry, catchError } from 'rxjs/operators';

this.workItemsService.listWorkItems().pipe(
  retry(2),  // Retry up to 2 times
  catchError(error => {
    console.error('Failed after retries:', error);
    return throwError(() => error);
  })
).subscribe(/* ... */);
```

### 7. WebSocket Reconnection

Handle WebSocket disconnections gracefully:

```typescript
export class WebSocketService {
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 5000;

  private handleDisconnect(token: string): void {
    if (this.reconnectAttempts < this.maxReconnectAttempts) {
      this.reconnectAttempts++;
      console.log(`Reconnecting... (${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
      setTimeout(() => this.connect(token), this.reconnectDelay);
    } else {
      console.error('Max reconnection attempts reached');
      // Notify user
    }
  }
}
```

## Troubleshooting

### CORS Issues

If you encounter CORS errors:

1. Ensure the API server has CORS configured for your origin
2. Check that credentials are being sent: `withCredentials: true`
3. Verify the API URL in environment configuration

### Authentication Failures

If authentication fails:

1. Check Keycloak configuration (realm, client ID)
2. Verify JWT token is being sent in Authorization header
3. Check token expiration and refresh logic
4. Ensure Keycloak is running and accessible

### WebSocket Connection Issues

If WebSocket fails to connect:

1. Verify WebSocket URL (ws:// for HTTP, wss:// for HTTPS)
2. Check that JWT token is included in connection URL
3. Ensure WebSocket endpoint is accessible
4. Check browser console for connection errors

### Type Errors

If you encounter TypeScript errors:

1. Ensure api-types.ts is up to date
2. Regenerate types: `make openapi-generate`
3. Check that imports are correct
4. Verify TypeScript version compatibility

## Additional Resources

- [OpenAPI Specification](../api/openapi/main.yaml)
- [API Documentation](./api/api-documentation.html)
- [TypeScript Types](../api/frontend/api-types.ts)
- [Angular Service Examples](../api/frontend/angular-services.example.ts)
- [WebSocket Examples](../api/frontend/websocket.example.ts)
- [Component Examples](../api/frontend/component.example.ts)
- [NATS Documentation](./nats.md)
- [Keycloak Documentation](https://www.keycloak.org/documentation)

## Support

For issues or questions:
- Review this integration guide
- Check the API documentation
- Examine the example code in `api/frontend/`
- Contact the development team

## License

MIT License - See LICENSE file for details
