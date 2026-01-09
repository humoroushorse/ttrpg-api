/**
 * Angular Service Examples for Sprint Management API
 * 
 * These examples demonstrate how to integrate the Sprint Management API
 * with Angular applications using HttpClient and RxJS.
 * 
 * @version 1.0.0
 */

import { Injectable } from '@angular/core';
import { HttpClient, HttpParams, HttpHeaders } from '@angular/common/http';
import { Observable, throwError } from 'rxjs';
import { catchError, map, retry } from 'rxjs/operators';
import { environment } from '../environments/environment';

// Import generated types
import {
  WorkItem,
  WorkItemDetail,
  WorkItemListResponse,
  CreateWorkItemRequest,
  UpdateWorkItemRequest,
  WorkItemListParams,
  Sprint,
  SprintDetail,
  SprintListResponse,
  CreateSprintRequest,
  UpdateSprintRequest,
  SprintListParams,
  SprintMetrics,
  BurndownData,
  VelocityData,
  TeamVelocityHistory,
  SprintForecast,
  WorkItemDependency,
  DependencyListResponse,
  CreateDependencyRequest,
  DependencyType,
  Comment,
  CommentListResponse,
  CreateCommentRequest,
  UpdateCommentRequest,
  SearchWorkItemsRequest,
  ErrorResponse
} from './api-types';

// ============================================================================
// Work Items Service
// ============================================================================

@Injectable({
  providedIn: 'root'
})
export class WorkItemsService {
  private readonly baseUrl = `${environment.apiUrl}/api/v1/workitems`;

  constructor(private http: HttpClient) {}

  /**
   * List work items with optional filtering and pagination
   * 
   * @param params Query parameters for filtering and pagination
   * @returns Observable of paginated work item list
   * 
   * @example
   * ```typescript
   * this.workItemsService.listWorkItems({
   *   status: WorkItemStatus.InProgress,
   *   assignee_id: 'user-123',
   *   limit: 20
   * }).subscribe(response => {
   *   console.log('Work items:', response.items);
   *   console.log('Has more:', response.pagination.has_more);
   * });
   * ```
   */
  listWorkItems(params?: WorkItemListParams): Observable<WorkItemListResponse> {
    let httpParams = new HttpParams();
    
    if (params) {
      Object.keys(params).forEach(key => {
        const value = params[key as keyof WorkItemListParams];
        if (value !== undefined && value !== null) {
          httpParams = httpParams.set(key, value.toString());
        }
      });
    }
    
    return this.http.get<WorkItemListResponse>(this.baseUrl, { params: httpParams })
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Get a specific work item by ID with full details
   * 
   * @param id Work item ID
   * @returns Observable of work item detail
   * 
   * @example
   * ```typescript
   * this.workItemsService.getWorkItem('work-item-123').subscribe(workItem => {
   *   console.log('Work item:', workItem);
   *   console.log('Dependencies:', workItem.dependencies);
   *   console.log('Comments:', workItem.comments);
   * });
   * ```
   */
  getWorkItem(id: string): Observable<WorkItemDetail> {
    return this.http.get<WorkItemDetail>(`${this.baseUrl}/${id}`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Create a new work item
   * 
   * @param request Work item creation request
   * @returns Observable of created work item
   * 
   * @example
   * ```typescript
   * const request: CreateWorkItemRequest = {
   *   type: WorkItemType.Story,
   *   title: 'User login feature',
   *   description: 'Implement user authentication with Keycloak',
   *   priority: PriorityLevel.High,
   *   story_points: 5,
   *   assignee_id: 'user-123'
   * };
   * 
   * this.workItemsService.createWorkItem(request).subscribe(workItem => {
   *   console.log('Created work item:', workItem);
   * });
   * ```
   */
  createWorkItem(request: CreateWorkItemRequest): Observable<WorkItem> {
    return this.http.post<WorkItem>(this.baseUrl, request)
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Update an existing work item
   * 
   * @param id Work item ID
   * @param request Work item update request
   * @returns Observable of updated work item
   * 
   * @example
   * ```typescript
   * this.workItemsService.updateWorkItem('work-item-123', {
   *   status: WorkItemStatus.Done,
   *   story_points: 8
   * }).subscribe(workItem => {
   *   console.log('Updated work item:', workItem);
   * });
   * ```
   */
  updateWorkItem(id: string, request: UpdateWorkItemRequest): Observable<WorkItem> {
    return this.http.put<WorkItem>(`${this.baseUrl}/${id}`, request)
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Delete a work item (soft delete)
   * 
   * @param id Work item ID
   * @returns Observable of void
   * 
   * @example
   * ```typescript
   * this.workItemsService.deleteWorkItem('work-item-123').subscribe(() => {
   *   console.log('Work item deleted');
   * });
   * ```
   */
  deleteWorkItem(id: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${id}`)
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Get child work items (sub-tasks)
   * 
   * @param id Parent work item ID
   * @returns Observable of paginated work item list
   * 
   * @example
   * ```typescript
   * this.workItemsService.getChildWorkItems('epic-123').subscribe(response => {
   *   console.log('Child work items:', response.items);
   * });
   * ```
   */
  getChildWorkItems(id: string): Observable<WorkItemListResponse> {
    return this.http.get<WorkItemListResponse>(`${this.baseUrl}/${id}/children`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Advanced search with multiple criteria
   * 
   * @param request Search request with filters
   * @returns Observable of paginated work item list
   * 
   * @example
   * ```typescript
   * const searchRequest: SearchWorkItemsRequest = {
   *   query: 'authentication',
   *   types: [WorkItemType.Story, WorkItemType.Defect],
   *   statuses: [WorkItemStatus.InProgress, WorkItemStatus.InReview],
   *   priorities: [PriorityLevel.High, PriorityLevel.Critical],
   *   created_after: '2024-01-01',
   *   limit: 50
   * };
   * 
   * this.workItemsService.searchWorkItems(searchRequest).subscribe(response => {
   *   console.log('Search results:', response.items);
   * });
   * ```
   */
  searchWorkItems(request: SearchWorkItemsRequest): Observable<WorkItemListResponse> {
    return this.http.post<WorkItemListResponse>(`${this.baseUrl}/search`, request)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Get work item dependencies
   * 
   * @param id Work item ID
   * @returns Observable of dependency list
   */
  getDependencies(id: string): Observable<DependencyListResponse> {
    return this.http.get<DependencyListResponse>(`${this.baseUrl}/${id}/dependencies`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Create a dependency between work items
   * 
   * @param id Source work item ID
   * @param request Dependency creation request
   * @returns Observable of created dependency
   * 
   * @example
   * ```typescript
   * this.workItemsService.createDependency('work-item-123', {
   *   target_id: 'work-item-456',
   *   dependency_type: DependencyType.Blocks
   * }).subscribe(dependency => {
   *   console.log('Created dependency:', dependency);
   * });
   * ```
   */
  createDependency(id: string, request: CreateDependencyRequest): Observable<WorkItemDependency> {
    return this.http.post<WorkItemDependency>(`${this.baseUrl}/${id}/dependencies`, request)
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Delete a dependency
   * 
   * @param id Source work item ID
   * @param dependencyId Dependency ID
   * @returns Observable of void
   */
  deleteDependency(id: string, dependencyId: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${id}/dependencies/${dependencyId}`)
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Handle HTTP errors
   */
  private handleError(error: any): Observable<never> {
    let errorMessage = 'An error occurred';
    
    if (error.error && error.error.error) {
      const errorResponse: ErrorResponse = error.error;
      errorMessage = errorResponse.error.message;
      console.error('API Error:', {
        code: errorResponse.error.code,
        message: errorResponse.error.message,
        trace_id: errorResponse.trace_id
      });
    } else {
      console.error('HTTP Error:', error);
    }
    
    return throwError(() => new Error(errorMessage));
  }
}

// ============================================================================
// Sprints Service
// ============================================================================

@Injectable({
  providedIn: 'root'
})
export class SprintsService {
  private readonly baseUrl = `${environment.apiUrl}/api/v1/sprints`;

  constructor(private http: HttpClient) {}

  /**
   * List sprints with optional filtering
   * 
   * @param params Query parameters for filtering and pagination
   * @returns Observable of paginated sprint list
   * 
   * @example
   * ```typescript
   * this.sprintsService.listSprints({
   *   status: SprintStatus.Active,
   *   limit: 10
   * }).subscribe(response => {
   *   console.log('Active sprints:', response.items);
   * });
   * ```
   */
  listSprints(params?: SprintListParams): Observable<SprintListResponse> {
    let httpParams = new HttpParams();
    
    if (params) {
      Object.keys(params).forEach(key => {
        const value = params[key as keyof SprintListParams];
        if (value !== undefined && value !== null) {
          httpParams = httpParams.set(key, value.toString());
        }
      });
    }
    
    return this.http.get<SprintListResponse>(this.baseUrl, { params: httpParams })
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Get a specific sprint by ID with full details
   * 
   * @param id Sprint ID
   * @returns Observable of sprint detail
   * 
   * @example
   * ```typescript
   * this.sprintsService.getSprint('sprint-123').subscribe(sprint => {
   *   console.log('Sprint:', sprint);
   *   console.log('Work items:', sprint.work_items);
   *   console.log('Metrics:', sprint.metrics);
   * });
   * ```
   */
  getSprint(id: string): Observable<SprintDetail> {
    return this.http.get<SprintDetail>(`${this.baseUrl}/${id}`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Create a new sprint
   * 
   * @param request Sprint creation request
   * @returns Observable of created sprint
   * 
   * @example
   * ```typescript
   * const request: CreateSprintRequest = {
   *   name: 'Sprint 15',
   *   description: 'Q1 2024 Sprint',
   *   start_date: '2024-01-15',
   *   end_date: '2024-01-29',
   *   capacity_points: 50
   * };
   * 
   * this.sprintsService.createSprint(request).subscribe(sprint => {
   *   console.log('Created sprint:', sprint);
   * });
   * ```
   */
  createSprint(request: CreateSprintRequest): Observable<Sprint> {
    return this.http.post<Sprint>(this.baseUrl, request)
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Update an existing sprint
   * 
   * @param id Sprint ID
   * @param request Sprint update request
   * @returns Observable of updated sprint
   * 
   * @example
   * ```typescript
   * this.sprintsService.updateSprint('sprint-123', {
   *   status: SprintStatus.Active,
   *   capacity_points: 60
   * }).subscribe(sprint => {
   *   console.log('Updated sprint:', sprint);
   * });
   * ```
   */
  updateSprint(id: string, request: UpdateSprintRequest): Observable<Sprint> {
    return this.http.put<Sprint>(`${this.baseUrl}/${id}`, request)
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Close a sprint (moves incomplete work items to backlog)
   * 
   * @param id Sprint ID
   * @returns Observable of close result
   * 
   * @example
   * ```typescript
   * this.sprintsService.closeSprint('sprint-123').subscribe(result => {
   *   console.log('Sprint closed:', result);
   *   console.log('Moved to backlog:', result.moved_items);
   * });
   * ```
   */
  closeSprint(id: string): Observable<any> {
    return this.http.post(`${this.baseUrl}/${id}/close`, {})
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Get sprint metrics
   * 
   * @param id Sprint ID
   * @returns Observable of sprint metrics
   * 
   * @example
   * ```typescript
   * this.sprintsService.getSprintMetrics('sprint-123').subscribe(metrics => {
   *   console.log('Completion:', metrics.completion_percentage + '%');
   *   console.log('Velocity:', metrics.velocity);
   *   console.log('Days remaining:', metrics.days_remaining);
   * });
   * ```
   */
  getSprintMetrics(id: string): Observable<SprintMetrics> {
    return this.http.get<SprintMetrics>(`${this.baseUrl}/${id}/metrics`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Get sprint burndown chart data
   * 
   * @param id Sprint ID
   * @returns Observable of burndown data
   * 
   * @example
   * ```typescript
   * this.sprintsService.getSprintBurndown('sprint-123').subscribe(data => {
   *   console.log('Burndown data points:', data.data_points);
   *   console.log('Ideal line:', data.ideal_line);
   *   // Use this data to render a chart
   * });
   * ```
   */
  getSprintBurndown(id: string): Observable<BurndownData> {
    return this.http.get<BurndownData>(`${this.baseUrl}/${id}/burndown`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Get sprint velocity
   * 
   * @param id Sprint ID
   * @returns Observable of velocity data
   * 
   * @example
   * ```typescript
   * this.sprintsService.getSprintVelocity('sprint-123').subscribe(velocity => {
   *   console.log('Velocity:', velocity.velocity);
   *   console.log('Completion rate:', velocity.completion_rate);
   * });
   * ```
   */
  getSprintVelocity(id: string): Observable<VelocityData> {
    return this.http.get<VelocityData>(`${this.baseUrl}/${id}/velocity`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Get team velocity history across multiple sprints
   * 
   * @returns Observable of team velocity history
   * 
   * @example
   * ```typescript
   * this.sprintsService.getTeamVelocityHistory().subscribe(history => {
   *   console.log('Average velocity:', history.average_velocity);
   *   console.log('Trend:', history.trend);
   *   console.log('Historical data:', history.sprints);
   * });
   * ```
   */
  getTeamVelocityHistory(): Observable<TeamVelocityHistory> {
    return this.http.get<TeamVelocityHistory>(`${this.baseUrl}/planning/velocity`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Get sprint capacity forecast based on historical data
   * 
   * @returns Observable of sprint forecast
   * 
   * @example
   * ```typescript
   * this.sprintsService.getSprintForecast().subscribe(forecast => {
   *   console.log('Forecasted velocity:', forecast.forecasted_velocity);
   *   console.log('Recommended capacity:', forecast.recommended_capacity);
   *   console.log('Confidence:', forecast.confidence);
   * });
   * ```
   */
  getSprintForecast(): Observable<SprintForecast> {
    return this.http.get<SprintForecast>(`${this.baseUrl}/planning/forecast`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Get work items in a sprint
   * 
   * @param id Sprint ID
   * @returns Observable of work item list
   */
  getSprintWorkItems(id: string): Observable<WorkItemListResponse> {
    return this.http.get<WorkItemListResponse>(`${this.baseUrl}/${id}/workitems`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Handle HTTP errors
   */
  private handleError(error: any): Observable<never> {
    let errorMessage = 'An error occurred';
    
    if (error.error && error.error.error) {
      const errorResponse: ErrorResponse = error.error;
      errorMessage = errorResponse.error.message;
      console.error('API Error:', {
        code: errorResponse.error.code,
        message: errorResponse.error.message,
        trace_id: errorResponse.trace_id
      });
    } else {
      console.error('HTTP Error:', error);
    }
    
    return throwError(() => new Error(errorMessage));
  }
}

// ============================================================================
// Comments Service
// ============================================================================

@Injectable({
  providedIn: 'root'
})
export class CommentsService {
  private readonly baseUrl = `${environment.apiUrl}/api/v1/workitems`;

  constructor(private http: HttpClient) {}

  /**
   * Get comments for a work item
   * 
   * @param workItemId Work item ID
   * @returns Observable of comment list
   */
  getComments(workItemId: string): Observable<CommentListResponse> {
    return this.http.get<CommentListResponse>(`${this.baseUrl}/${workItemId}/comments`)
      .pipe(
        retry(1),
        catchError(this.handleError)
      );
  }

  /**
   * Create a comment on a work item
   * 
   * @param workItemId Work item ID
   * @param request Comment creation request
   * @returns Observable of created comment
   * 
   * @example
   * ```typescript
   * this.commentsService.createComment('work-item-123', {
   *   content: 'This looks good, approved!'
   * }).subscribe(comment => {
   *   console.log('Created comment:', comment);
   * });
   * ```
   */
  createComment(workItemId: string, request: CreateCommentRequest): Observable<Comment> {
    return this.http.post<Comment>(`${this.baseUrl}/${workItemId}/comments`, request)
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Update a comment
   * 
   * @param workItemId Work item ID
   * @param commentId Comment ID
   * @param request Comment update request
   * @returns Observable of updated comment
   */
  updateComment(workItemId: string, commentId: string, request: UpdateCommentRequest): Observable<Comment> {
    return this.http.put<Comment>(`${this.baseUrl}/${workItemId}/comments/${commentId}`, request)
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Delete a comment (soft delete)
   * 
   * @param workItemId Work item ID
   * @param commentId Comment ID
   * @returns Observable of void
   */
  deleteComment(workItemId: string, commentId: string): Observable<void> {
    return this.http.delete<void>(`${this.baseUrl}/${workItemId}/comments/${commentId}`)
      .pipe(
        catchError(this.handleError)
      );
  }

  /**
   * Handle HTTP errors
   */
  private handleError(error: any): Observable<never> {
    let errorMessage = 'An error occurred';
    
    if (error.error && error.error.error) {
      const errorResponse: ErrorResponse = error.error;
      errorMessage = errorResponse.error.message;
    }
    
    return throwError(() => new Error(errorMessage));
  }
}

// ============================================================================
// Environment Configuration Example
// ============================================================================

/**
 * Example environment configuration
 * 
 * Create this in your Angular project:
 * - src/environments/environment.ts (development)
 * - src/environments/environment.prod.ts (production)
 */
export const environmentExample = {
  production: false,
  apiUrl: 'http://localhost:8082',
  wsUrl: 'ws://localhost:8082/ws',
  keycloakUrl: 'http://localhost:8080',
  keycloakRealm: 'sprint-management',
  keycloakClientId: 'sprint-ui'
};
