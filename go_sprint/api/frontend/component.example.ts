/**
 * Angular Component Examples
 * 
 * These examples demonstrate how to use the Sprint Management API services
 * in Angular components with real-time WebSocket updates.
 * 
 * @version 1.0.0
 */

import { Component, OnInit, OnDestroy } from '@angular/core';
import { FormBuilder, FormGroup, Validators } from '@angular/forms';
import { Subscription } from 'rxjs';

// Import services
import { WorkItemsService } from './angular-services.example';
import { SprintsService } from './angular-services.example';
import { WebSocketService, WebSocketState } from './websocket.example';

// Import types
import {
  WorkItem,
  WorkItemType,
  WorkItemStatus,
  PriorityLevel,
  CreateWorkItemRequest,
  Sprint,
  SprintStatus,
  SprintMetrics,
  BurndownData,
  WorkItemEventData
} from './api-types';

// ============================================================================
// Sprint Board Component Example
// ============================================================================

/**
 * Sprint board component with real-time updates
 * 
 * Features:
 * - Display work items grouped by status
 * - Real-time updates via WebSocket
 * - Drag and drop support (not shown in this example)
 * - Sprint metrics display
 */
@Component({
  selector: 'app-sprint-board',
  template: `
    <div class="sprint-board">
      <h1>{{ sprint?.name }}</h1>
      
      <!-- Sprint Metrics -->
      <div class="metrics" *ngIf="metrics">
        <div class="metric">
          <span>Completion:</span>
          <strong>{{ metrics.completion_percentage }}%</strong>
        </div>
        <div class="metric">
          <span>Velocity:</span>
          <strong>{{ metrics.velocity }} points</strong>
        </div>
        <div class="metric">
          <span>Days Remaining:</span>
          <strong>{{ metrics.days_remaining }}</strong>
        </div>
      </div>
      
      <!-- Work Item Columns -->
      <div class="columns">
        <div class="column" *ngFor="let status of statuses">
          <h3>{{ status }}</h3>
          <div class="work-items">
            <div class="work-item" *ngFor="let item of getWorkItemsByStatus(status)">
              <span class="type">{{ item.type }}</span>
              <h4>{{ item.title }}</h4>
              <p>{{ item.description }}</p>
              <span class="points" *ngIf="item.story_points">
                {{ item.story_points }} points
              </span>
            </div>
          </div>
        </div>
      </div>
    </div>
  `
})
export class SprintBoardComponent implements OnInit, OnDestroy {
  sprintId: string = 'sprint-123'; // From route params
  sprint: Sprint | null = null;
  workItems: WorkItem[] = [];
  metrics: SprintMetrics | null = null;
  
  statuses = Object.values(WorkItemStatus);
  
  private subscriptions: Subscription[] = [];

  constructor(
    private workItemsService: WorkItemsService,
    private sprintsService: SprintsService,
    private wsService: WebSocketService
  ) {}

  ngOnInit(): void {
    // Load sprint data
    this.loadSprint();
    this.loadWorkItems();
    this.loadMetrics();
    
    // Connect to WebSocket
    const token = this.getAuthToken(); // Get from your auth service
    this.wsService.connect(token);
    
    // Join sprint room
    this.wsService.joinRoom(`sprint:${this.sprintId}`);
    
    // Subscribe to real-time updates
    this.subscribeToUpdates();
  }

  ngOnDestroy(): void {
    // Leave room and cleanup
    this.wsService.leaveRoom(`sprint:${this.sprintId}`);
    this.subscriptions.forEach(sub => sub.unsubscribe());
  }

  private loadSprint(): void {
    this.sprintsService.getSprint(this.sprintId).subscribe({
      next: (sprint) => {
        this.sprint = sprint;
      },
      error: (error) => {
        console.error('Failed to load sprint:', error);
      }
    });
  }

  private loadWorkItems(): void {
    this.workItemsService.listWorkItems({
      sprint_id: this.sprintId,
      limit: 100
    }).subscribe({
      next: (response) => {
        this.workItems = response.items;
      },
      error: (error) => {
        console.error('Failed to load work items:', error);
      }
    });
  }

  private loadMetrics(): void {
    this.sprintsService.getSprintMetrics(this.sprintId).subscribe({
      next: (metrics) => {
        this.metrics = metrics;
      },
      error: (error) => {
        console.error('Failed to load metrics:', error);
      }
    });
  }

  private subscribeToUpdates(): void {
    // Work item created
    const createdSub = this.wsService.onWorkItemCreated().subscribe({
      next: (event: WorkItemEventData) => {
        if (event.work_item.sprint_id === this.sprintId) {
          this.workItems.push(event.work_item);
          this.loadMetrics(); // Refresh metrics
        }
      }
    });
    this.subscriptions.push(createdSub);
    
    // Work item updated
    const updatedSub = this.wsService.onWorkItemUpdated().subscribe({
      next: (event: WorkItemEventData) => {
        const index = this.workItems.findIndex(item => item.id === event.work_item.id);
        if (index !== -1) {
          this.workItems[index] = event.work_item;
          this.loadMetrics(); // Refresh metrics
        }
      }
    });
    this.subscriptions.push(updatedSub);
    
    // Work item status changed
    const statusSub = this.wsService.onWorkItemStatusChanged().subscribe({
      next: (event: WorkItemEventData) => {
        const index = this.workItems.findIndex(item => item.id === event.work_item.id);
        if (index !== -1) {
          this.workItems[index] = event.work_item;
          this.loadMetrics(); // Refresh metrics
        }
      }
    });
    this.subscriptions.push(statusSub);
  }

  getWorkItemsByStatus(status: WorkItemStatus): WorkItem[] {
    return this.workItems.filter(item => item.status === status);
  }

  private getAuthToken(): string {
    // Get JWT token from your auth service
    return localStorage.getItem('jwt_token') || '';
  }
}


// ============================================================================
// Work Item Create Component Example
// ============================================================================

/**
 * Work item creation form component
 * 
 * Features:
 * - Reactive form with validation
 * - Type-safe form handling
 * - Error handling and display
 */
@Component({
  selector: 'app-work-item-create',
  template: `
    <form [formGroup]="workItemForm" (ngSubmit)="onSubmit()">
      <h2>Create Work Item</h2>
      
      <div class="form-group">
        <label>Type</label>
        <select formControlName="type">
          <option *ngFor="let type of workItemTypes" [value]="type">
            {{ type }}
          </option>
        </select>
      </div>
      
      <div class="form-group">
        <label>Title</label>
        <input type="text" formControlName="title" />
        <div class="error" *ngIf="workItemForm.get('title')?.errors?.['required']">
          Title is required
        </div>
      </div>
      
      <div class="form-group">
        <label>Description</label>
        <textarea formControlName="description"></textarea>
      </div>
      
      <div class="form-group">
        <label>Priority</label>
        <select formControlName="priority">
          <option *ngFor="let priority of priorities" [value]="priority">
            {{ priority }}
          </option>
        </select>
      </div>
      
      <div class="form-group">
        <label>Story Points</label>
        <input type="number" formControlName="story_points" />
      </div>
      
      <button type="submit" [disabled]="!workItemForm.valid || isSubmitting">
        {{ isSubmitting ? 'Creating...' : 'Create Work Item' }}
      </button>
      
      <div class="error" *ngIf="errorMessage">
        {{ errorMessage }}
      </div>
    </form>
  `
})
export class WorkItemCreateComponent {
  workItemForm: FormGroup;
  workItemTypes = Object.values(WorkItemType);
  priorities = Object.values(PriorityLevel);
  isSubmitting = false;
  errorMessage: string | null = null;

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
      this.isSubmitting = true;
      this.errorMessage = null;
      
      const request: CreateWorkItemRequest = this.workItemForm.value;
      
      this.workItemsService.createWorkItem(request).subscribe({
        next: (workItem) => {
          console.log('Work item created:', workItem);
          this.isSubmitting = false;
          this.workItemForm.reset();
          // Navigate or show success message
        },
        error: (error) => {
          console.error('Failed to create work item:', error);
          this.errorMessage = error.message || 'Failed to create work item';
          this.isSubmitting = false;
        }
      });
    }
  }
}

// ============================================================================
// Burndown Chart Component Example
// ============================================================================

/**
 * Sprint burndown chart component
 * 
 * Features:
 * - Display burndown chart data
 * - Compare actual vs ideal burndown
 * - Integration with chart library (Chart.js, ngx-charts, etc.)
 */
@Component({
  selector: 'app-sprint-burndown',
  template: `
    <div class="burndown-chart">
      <h2>Sprint Burndown</h2>
      
      <div *ngIf="burndownData">
        <!-- Use your preferred chart library here -->
        <!-- Example with ngx-charts -->
        <ngx-charts-line-chart
          [results]="chartData"
          [xAxis]="true"
          [yAxis]="true"
          [legend]="true"
          [showXAxisLabel]="true"
          [showYAxisLabel]="true"
          xAxisLabel="Date"
          yAxisLabel="Story Points">
        </ngx-charts-line-chart>
        
        <div class="chart-info">
          <p>Initial Points: {{ burndownData.initial_points }}</p>
          <p>Start Date: {{ burndownData.start_date | date }}</p>
          <p>End Date: {{ burndownData.end_date | date }}</p>
        </div>
      </div>
      
      <div *ngIf="!burndownData && !isLoading">
        No burndown data available
      </div>
      
      <div *ngIf="isLoading">
        Loading burndown data...
      </div>
    </div>
  `
})
export class SprintBurndownComponent implements OnInit {
  sprintId: string = 'sprint-123'; // From route params
  burndownData: BurndownData | null = null;
  chartData: any[] = [];
  isLoading = false;

  constructor(private sprintsService: SprintsService) {}

  ngOnInit(): void {
    this.loadBurndownData();
  }

  private loadBurndownData(): void {
    this.isLoading = true;
    
    this.sprintsService.getSprintBurndown(this.sprintId).subscribe({
      next: (data) => {
        this.burndownData = data;
        this.prepareChartData();
        this.isLoading = false;
      },
      error: (error) => {
        console.error('Failed to load burndown data:', error);
        this.isLoading = false;
      }
    });
  }

  private prepareChartData(): void {
    if (!this.burndownData) return;
    
    // Transform data for chart library
    this.chartData = [
      {
        name: 'Actual',
        series: this.burndownData.data_points.map(point => ({
          name: point.date,
          value: point.remaining_points
        }))
      },
      {
        name: 'Ideal',
        series: this.burndownData.ideal_line.map(point => ({
          name: point.date,
          value: point.ideal_points
        }))
      }
    ];
  }
}

// ============================================================================
// Sprint List Component Example
// ============================================================================

/**
 * Sprint list component with filtering
 * 
 * Features:
 * - List sprints with pagination
 * - Filter by status
 * - Navigate to sprint details
 */
@Component({
  selector: 'app-sprint-list',
  template: `
    <div class="sprint-list">
      <h1>Sprints</h1>
      
      <!-- Filter -->
      <div class="filters">
        <label>Status:</label>
        <select [(ngModel)]="selectedStatus" (change)="onFilterChange()">
          <option value="">All</option>
          <option *ngFor="let status of sprintStatuses" [value]="status">
            {{ status }}
          </option>
        </select>
      </div>
      
      <!-- Sprint Cards -->
      <div class="sprints">
        <div class="sprint-card" *ngFor="let sprint of sprints" (click)="navigateToSprint(sprint.id)">
          <h3>{{ sprint.name }}</h3>
          <p>{{ sprint.description }}</p>
          <div class="sprint-info">
            <span class="status" [class]="sprint.status">{{ sprint.status }}</span>
            <span class="dates">
              {{ sprint.start_date | date }} - {{ sprint.end_date | date }}
            </span>
            <span class="points">
              {{ sprint.completed_points }} / {{ sprint.committed_points }} points
            </span>
          </div>
        </div>
      </div>
      
      <!-- Pagination -->
      <div class="pagination" *ngIf="hasMore">
        <button (click)="loadMore()" [disabled]="isLoading">
          {{ isLoading ? 'Loading...' : 'Load More' }}
        </button>
      </div>
    </div>
  `
})
export class SprintListComponent implements OnInit {
  sprints: Sprint[] = [];
  sprintStatuses = Object.values(SprintStatus);
  selectedStatus: string = '';
  nextCursor: string | undefined;
  hasMore = false;
  isLoading = false;

  constructor(private sprintsService: SprintsService) {}

  ngOnInit(): void {
    this.loadSprints();
  }

  private loadSprints(cursor?: string): void {
    this.isLoading = true;
    
    const params: any = {
      limit: 20
    };
    
    if (this.selectedStatus) {
      params.status = this.selectedStatus;
    }
    
    if (cursor) {
      params.cursor = cursor;
    }
    
    this.sprintsService.listSprints(params).subscribe({
      next: (response) => {
        if (cursor) {
          // Append to existing sprints
          this.sprints = [...this.sprints, ...response.items];
        } else {
          // Replace sprints
          this.sprints = response.items;
        }
        
        this.nextCursor = response.pagination.next_cursor;
        this.hasMore = response.pagination.has_more;
        this.isLoading = false;
      },
      error: (error) => {
        console.error('Failed to load sprints:', error);
        this.isLoading = false;
      }
    });
  }

  onFilterChange(): void {
    this.sprints = [];
    this.nextCursor = undefined;
    this.loadSprints();
  }

  loadMore(): void {
    if (this.nextCursor && !this.isLoading) {
      this.loadSprints(this.nextCursor);
    }
  }

  navigateToSprint(sprintId: string): void {
    // Use Angular Router to navigate
    // this.router.navigate(['/sprints', sprintId]);
    console.log('Navigate to sprint:', sprintId);
  }
}
