/**
 * TypeScript interfaces for Sprint Management API
 * Generated from OpenAPI 3.0 specifications
 * 
 * @version 1.0.0
 * @description Type-safe interfaces for Angular/React/Vue frontend integration
 */

// ============================================================================
// Enums
// ============================================================================

/**
 * Work item types
 */
export enum WorkItemType {
  Epic = 'epic',
  Story = 'story',
  Defect = 'defect'
}

/**
 * Work item status values
 */
export enum WorkItemStatus {
  Todo = 'todo',
  InProgress = 'in_progress',
  InReview = 'in_review',
  Done = 'done',
  Blocked = 'blocked'
}

/**
 * Priority levels
 */
export enum PriorityLevel {
  Low = 'low',
  Medium = 'medium',
  High = 'high',
  Critical = 'critical'
}

/**
 * Sprint status values
 */
export enum SprintStatus {
  Planned = 'planned',
  Active = 'active',
  Completed = 'completed',
  Cancelled = 'cancelled'
}

/**
 * Dependency types between work items
 */
export enum DependencyType {
  Blocks = 'blocks',
  IsBlockedBy = 'is_blocked_by',
  RelatesTo = 'relates_to',
  Duplicates = 'duplicates'
}

// ============================================================================
// Core Models
// ============================================================================

/**
 * Work Item model
 */
export interface WorkItem {
  /** Unique identifier */
  id: string;
  
  /** Work item type */
  type: WorkItemType;
  
  /** Title (1-255 characters) */
  title: string;
  
  /** Detailed description */
  description: string;
  
  /** Current status */
  status: WorkItemStatus;
  
  /** Priority level */
  priority: PriorityLevel;
  
  /** Story point estimation (optional) */
  story_points?: number;
  
  /** Assigned user ID (optional) */
  assignee_id?: string;
  
  /** Reporter user ID */
  reporter_id: string;
  
  /** Parent work item ID (for sub-tasks) */
  parent_id?: string;
  
  /** Sprint ID if assigned to a sprint */
  sprint_id?: string;
  
  /** Creation timestamp (ISO 8601) */
  created_at: string;
  
  /** Last update timestamp (ISO 8601) */
  updated_at: string;
  
  /** Soft delete timestamp (ISO 8601, null if not deleted) */
  deleted_at?: string;
  
  /** User ID who deleted the item */
  deleted_by?: string;
}

/**
 * Detailed work item with relationships
 */
export interface WorkItemDetail extends WorkItem {
  /** Child work items (sub-tasks) */
  children?: WorkItem[];
  
  /** Dependencies on other work items */
  dependencies?: WorkItemDependency[];
  
  /** Comments on this work item */
  comments?: Comment[];
  
  /** Activity history */
  activity?: ActivityLog[];
}

/**
 * Sprint model
 */
export interface Sprint {
  /** Unique identifier */
  id: string;
  
  /** Sprint name */
  name: string;
  
  /** Sprint description */
  description?: string;
  
  /** Sprint status */
  status: SprintStatus;
  
  /** Start date (ISO 8601 date) */
  start_date: string;
  
  /** End date (ISO 8601 date) */
  end_date: string;
  
  /** Sprint capacity in story points */
  capacity_points?: number;
  
  /** Committed story points */
  committed_points: number;
  
  /** Completed story points */
  completed_points: number;
  
  /** User ID who created the sprint */
  created_by: string;
  
  /** Creation timestamp (ISO 8601) */
  created_at: string;
  
  /** Last update timestamp (ISO 8601) */
  updated_at: string;
  
  /** Soft delete timestamp (ISO 8601, null if not deleted) */
  deleted_at?: string;
  
  /** User ID who deleted the sprint */
  deleted_by?: string;
}

/**
 * Detailed sprint with work items
 */
export interface SprintDetail extends Sprint {
  /** Work items in this sprint */
  work_items?: WorkItem[];
  
  /** Sprint metrics */
  metrics?: SprintMetrics;
}

/**
 * Work item dependency
 */
export interface WorkItemDependency {
  /** Unique identifier */
  id: string;
  
  /** Source work item ID */
  source_id: string;
  
  /** Target work item ID */
  target_id: string;
  
  /** Dependency type */
  dependency_type: DependencyType;
  
  /** Creation timestamp (ISO 8601) */
  created_at: string;
  
  /** User ID who created the dependency */
  created_by: string;
  
  /** Source work item details (optional) */
  source?: WorkItem;
  
  /** Target work item details (optional) */
  target?: WorkItem;
}

/**
 * Comment on a work item
 */
export interface Comment {
  /** Unique identifier */
  id: string;
  
  /** Work item ID */
  work_item_id: string;
  
  /** Author user ID */
  author_id: string;
  
  /** Comment content */
  content: string;
  
  /** Creation timestamp (ISO 8601) */
  created_at: string;
  
  /** Last update timestamp (ISO 8601) */
  updated_at: string;
  
  /** Soft delete timestamp (ISO 8601, null if not deleted) */
  deleted_at?: string;
  
  /** User ID who deleted the comment */
  deleted_by?: string;
}

/**
 * Activity log entry
 */
export interface ActivityLog {
  /** Unique identifier */
  id: string;
  
  /** Entity type (work_item, sprint, comment) */
  entity_type: string;
  
  /** Entity ID */
  entity_id: string;
  
  /** Action performed (created, updated, deleted, status_changed) */
  action: string;
  
  /** User ID who performed the action */
  user_id: string;
  
  /** Changes made (before/after values for updates) */
  changes?: Record<string, any>;
  
  /** Trace ID for request correlation */
  trace_id?: string;
  
  /** Timestamp (ISO 8601) */
  created_at: string;
}

// ============================================================================
// Request/Response Types
// ============================================================================

/**
 * Create work item request
 */
export interface CreateWorkItemRequest {
  /** Work item type */
  type: WorkItemType;
  
  /** Title (1-255 characters) */
  title: string;
  
  /** Detailed description */
  description: string;
  
  /** Priority level */
  priority: PriorityLevel;
  
  /** Story point estimation (optional) */
  story_points?: number;
  
  /** Assigned user ID (optional) */
  assignee_id?: string;
  
  /** Parent work item ID (for sub-tasks) */
  parent_id?: string;
  
  /** Sprint ID if assigning to a sprint */
  sprint_id?: string;
}

/**
 * Update work item request
 */
export interface UpdateWorkItemRequest {
  /** Title (1-255 characters) */
  title?: string;
  
  /** Detailed description */
  description?: string;
  
  /** Current status */
  status?: WorkItemStatus;
  
  /** Priority level */
  priority?: PriorityLevel;
  
  /** Story point estimation */
  story_points?: number;
  
  /** Assigned user ID */
  assignee_id?: string;
  
  /** Sprint ID */
  sprint_id?: string;
}

/**
 * Create sprint request
 */
export interface CreateSprintRequest {
  /** Sprint name */
  name: string;
  
  /** Sprint description */
  description?: string;
  
  /** Start date (ISO 8601 date) */
  start_date: string;
  
  /** End date (ISO 8601 date) */
  end_date: string;
  
  /** Sprint capacity in story points */
  capacity_points?: number;
}

/**
 * Update sprint request
 */
export interface UpdateSprintRequest {
  /** Sprint name */
  name?: string;
  
  /** Sprint description */
  description?: string;
  
  /** Sprint status */
  status?: SprintStatus;
  
  /** Start date (ISO 8601 date) */
  start_date?: string;
  
  /** End date (ISO 8601 date) */
  end_date?: string;
  
  /** Sprint capacity in story points */
  capacity_points?: number;
}

/**
 * Create dependency request
 */
export interface CreateDependencyRequest {
  /** Target work item ID */
  target_id: string;
  
  /** Dependency type */
  dependency_type: DependencyType;
}

/**
 * Create comment request
 */
export interface CreateCommentRequest {
  /** Comment content */
  content: string;
}

/**
 * Update comment request
 */
export interface UpdateCommentRequest {
  /** Comment content */
  content: string;
}

/**
 * Search work items request
 */
export interface SearchWorkItemsRequest {
  /** Search query string */
  query?: string;
  
  /** Work item types to filter */
  types?: WorkItemType[];
  
  /** Statuses to filter */
  statuses?: WorkItemStatus[];
  
  /** Priorities to filter */
  priorities?: PriorityLevel[];
  
  /** Assignee IDs to filter */
  assignee_ids?: string[];
  
  /** Sprint IDs to filter */
  sprint_ids?: string[];
  
  /** Date range filter - created after */
  created_after?: string;
  
  /** Date range filter - created before */
  created_before?: string;
  
  /** Pagination cursor */
  cursor?: string;
  
  /** Page size limit */
  limit?: number;
}

// ============================================================================
// Pagination
// ============================================================================

/**
 * Pagination information
 */
export interface PaginationInfo {
  /** Cursor for next page (null if no more pages) */
  next_cursor?: string;
  
  /** Whether there are more results */
  has_more: boolean;
  
  /** Total count (optional, may not always be available) */
  total_count?: number;
}

/**
 * Paginated work item list response
 */
export interface WorkItemListResponse {
  /** Work items */
  items: WorkItem[];
  
  /** Pagination information */
  pagination: PaginationInfo;
}

/**
 * Paginated sprint list response
 */
export interface SprintListResponse {
  /** Sprints */
  items: Sprint[];
  
  /** Pagination information */
  pagination: PaginationInfo;
}

/**
 * Paginated dependency list response
 */
export interface DependencyListResponse {
  /** Dependencies */
  items: WorkItemDependency[];
  
  /** Pagination information */
  pagination: PaginationInfo;
}

/**
 * Paginated comment list response
 */
export interface CommentListResponse {
  /** Comments */
  items: Comment[];
  
  /** Pagination information */
  pagination: PaginationInfo;
}

// ============================================================================
// Metrics and Analytics
// ============================================================================

/**
 * Sprint metrics
 */
export interface SprintMetrics {
  /** Sprint ID */
  sprint_id: string;
  
  /** Total work items in sprint */
  total_work_items: number;
  
  /** Completed work items */
  completed_work_items: number;
  
  /** In-progress work items */
  in_progress_work_items: number;
  
  /** Blocked work items */
  blocked_work_items: number;
  
  /** Total committed story points */
  committed_points: number;
  
  /** Completed story points */
  completed_points: number;
  
  /** Remaining story points */
  remaining_points: number;
  
  /** Completion percentage (0-100) */
  completion_percentage: number;
  
  /** Sprint velocity (completed points) */
  velocity: number;
  
  /** Days remaining in sprint */
  days_remaining: number;
  
  /** Days elapsed in sprint */
  days_elapsed: number;
  
  /** Total sprint duration in days */
  total_days: number;
}

/**
 * Burndown chart data point
 */
export interface BurndownDataPoint {
  /** Date (ISO 8601 date) */
  date: string;
  
  /** Remaining story points */
  remaining_points: number;
  
  /** Ideal remaining points for this date */
  ideal_points: number;
}

/**
 * Burndown chart data
 */
export interface BurndownData {
  /** Sprint ID */
  sprint_id: string;
  
  /** Data points for each day */
  data_points: BurndownDataPoint[];
  
  /** Ideal burndown line */
  ideal_line: BurndownDataPoint[];
  
  /** Sprint start date */
  start_date: string;
  
  /** Sprint end date */
  end_date: string;
  
  /** Initial committed points */
  initial_points: number;
}

/**
 * Velocity data
 */
export interface VelocityData {
  /** Sprint ID */
  sprint_id: string;
  
  /** Sprint name */
  sprint_name: string;
  
  /** Velocity (completed story points) */
  velocity: number;
  
  /** Committed points */
  committed_points: number;
  
  /** Completion rate (0-1) */
  completion_rate: number;
}

/**
 * Team velocity history
 */
export interface TeamVelocityHistory {
  /** Historical velocity data per sprint */
  sprints: VelocityData[];
  
  /** Average velocity across all sprints */
  average_velocity: number;
  
  /** Median velocity */
  median_velocity: number;
  
  /** Velocity trend (increasing, decreasing, stable) */
  trend: string;
}

/**
 * Sprint forecast
 */
export interface SprintForecast {
  /** Forecasted velocity based on history */
  forecasted_velocity: number;
  
  /** Confidence level (0-1) */
  confidence: number;
  
  /** Number of sprints used for forecast */
  historical_sprints_count: number;
  
  /** Recommended sprint capacity */
  recommended_capacity: number;
}

// ============================================================================
// Error Handling
// ============================================================================

/**
 * Validation error for a specific field
 */
export interface ValidationError {
  /** Field name */
  field: string;
  
  /** Error code */
  code: string;
  
  /** Error message */
  message: string;
}

/**
 * Error detail
 */
export interface ErrorDetail {
  /** Error code */
  code: string;
  
  /** Error message */
  message: string;
  
  /** Additional error details */
  details?: Record<string, any>;
  
  /** Validation errors (for 400 Bad Request) */
  validation_errors?: ValidationError[];
}

/**
 * Error response
 */
export interface ErrorResponse {
  /** Error details */
  error: ErrorDetail;
  
  /** Trace ID for debugging */
  trace_id: string;
  
  /** Timestamp (ISO 8601) */
  timestamp: string;
}

// ============================================================================
// WebSocket Messages
// ============================================================================

/**
 * WebSocket message types
 */
export enum WebSocketMessageType {
  WorkItemCreated = 'workitem.created',
  WorkItemUpdated = 'workitem.updated',
  WorkItemDeleted = 'workitem.deleted',
  WorkItemStatusChanged = 'workitem.status_changed',
  SprintCreated = 'sprint.created',
  SprintUpdated = 'sprint.updated',
  SprintClosed = 'sprint.closed',
  SprintStatusChanged = 'sprint.status_changed',
  CommentAdded = 'comment.added',
  CommentUpdated = 'comment.updated',
  CommentDeleted = 'comment.deleted',
  JoinRoom = 'join_room',
  LeaveRoom = 'leave_room',
  Error = 'error'
}

/**
 * WebSocket message
 */
export interface WebSocketMessage<T = any> {
  /** Message type */
  type: WebSocketMessageType | string;
  
  /** Room identifier (sprint:id, project:id) */
  room?: string;
  
  /** Message payload */
  data: T;
  
  /** Trace ID for correlation */
  trace_id: string;
  
  /** Timestamp (ISO 8601) */
  timestamp: string;
}

/**
 * WebSocket work item event data
 */
export interface WorkItemEventData {
  /** Work item */
  work_item: WorkItem;
  
  /** Previous status (for status changes) */
  previous_status?: WorkItemStatus;
  
  /** User ID who made the change */
  user_id: string;
}

/**
 * WebSocket sprint event data
 */
export interface SprintEventData {
  /** Sprint */
  sprint: Sprint;
  
  /** Previous status (for status changes) */
  previous_status?: SprintStatus;
  
  /** User ID who made the change */
  user_id: string;
}

/**
 * WebSocket comment event data
 */
export interface CommentEventData {
  /** Comment */
  comment: Comment;
  
  /** Work item ID */
  work_item_id: string;
  
  /** User ID who made the change */
  user_id: string;
}

// ============================================================================
// Query Parameters
// ============================================================================

/**
 * Work item list query parameters
 */
export interface WorkItemListParams {
  /** Filter by work item type */
  type?: WorkItemType;
  
  /** Filter by status */
  status?: WorkItemStatus;
  
  /** Filter by assignee ID */
  assignee_id?: string;
  
  /** Filter by sprint ID */
  sprint_id?: string;
  
  /** Filter by priority */
  priority?: PriorityLevel;
  
  /** Search query */
  search?: string;
  
  /** Pagination cursor */
  cursor?: string;
  
  /** Page size limit (default: 50, max: 100) */
  limit?: number;
}

/**
 * Sprint list query parameters
 */
export interface SprintListParams {
  /** Filter by status */
  status?: SprintStatus;
  
  /** Filter sprints starting after this date */
  start_date_after?: string;
  
  /** Filter sprints starting before this date */
  start_date_before?: string;
  
  /** Pagination cursor */
  cursor?: string;
  
  /** Page size limit (default: 50, max: 100) */
  limit?: number;
}

// ============================================================================
// Type Guards
// ============================================================================

/**
 * Type guard to check if a response is an error
 */
export function isErrorResponse(response: any): response is ErrorResponse {
  return response && response.error && response.trace_id;
}

/**
 * Type guard to check if a work item is an epic
 */
export function isEpic(workItem: WorkItem): boolean {
  return workItem.type === WorkItemType.Epic;
}

/**
 * Type guard to check if a work item is a story
 */
export function isStory(workItem: WorkItem): boolean {
  return workItem.type === WorkItemType.Story;
}

/**
 * Type guard to check if a work item is a defect
 */
export function isDefect(workItem: WorkItem): boolean {
  return workItem.type === WorkItemType.Defect;
}

/**
 * Type guard to check if a sprint is active
 */
export function isActiveSprint(sprint: Sprint): boolean {
  return sprint.status === SprintStatus.Active;
}

/**
 * Type guard to check if a sprint is completed
 */
export function isCompletedSprint(sprint: Sprint): boolean {
  return sprint.status === SprintStatus.Completed;
}
