/**
 * WebSocket Integration Example for Sprint Management API
 * 
 * This example demonstrates real-time communication using WebSockets
 * for live updates on work items, sprints, and comments.
 * 
 * @version 1.0.0
 */

import { Injectable, OnDestroy } from '@angular/core';
import { Observable, Subject, BehaviorSubject } from 'rxjs';
import { filter } from 'rxjs/operators';
import { environment } from '../environments/environment';

// Import types
import {
  WebSocketMessage,
  WebSocketMessageType,
  WorkItemEventData,
  SprintEventData,
  CommentEventData
} from './api-types';

/**
 * WebSocket connection states
 */
export enum WebSocketState {
  Connecting = 'CONNECTING',
  Connected = 'CONNECTED',
  Disconnected = 'DISCONNECTED',
  Error = 'ERROR'
}

/**
 * WebSocket Service for real-time updates
 * 
 * @example
 * ```typescript
 * // In your component
 * constructor(private wsService: WebSocketService) {}
 * 
 * ngOnInit() {
 *   // Connect to WebSocket
 *   this.wsService.connect();
 *   
 *   // Subscribe to work item updates
 *   this.wsService.onWorkItemUpdated().subscribe(event => {
 *     console.log('Work item updated:', event.work_item);
 *   });
 * }
 * ```
 */
@Injectable({
  providedIn: 'root'
})
export class WebSocketService implements OnDestroy {
  private socket: WebSocket | null = null;
  private messageSubject = new Subject<WebSocketMessage>();
  private stateSubject = new BehaviorSubject<WebSocketState>(WebSocketState.Disconnected);
  private reconnectAttempts = 0;
  private maxReconnectAttempts = 5;
  private reconnectDelay = 5000; // 5 seconds

  constructor() {}

  /**
   * Connect to WebSocket server
   * Automatically includes JWT token from AuthService
   * 
   * @param token JWT authentication token
   */
  connect(token: string): void {
    if (this.socket && this.socket.readyState === WebSocket.OPEN) {
      console.log('WebSocket already connected');
      return;
    }

    const wsUrl = `${environment.wsUrl}?token=${encodeURIComponent(token)}`;
    
    try {
      this.stateSubject.next(WebSocketState.Connecting);
      this.socket = new WebSocket(wsUrl);
      
      this.socket.onopen = () => {
        console.log('WebSocket connected');
        this.stateSubject.next(WebSocketState.Connected);
        this.reconnectAttempts = 0;
      };
      
      this.socket.onmessage = (event) => {
        try {
          const message: WebSocketMessage = JSON.parse(event.data);
          this.messageSubject.next(message);
        } catch (error) {
          console.error('Failed to parse WebSocket message:', error);
        }
      };
      
      this.socket.onerror = (error) => {
        console.error('WebSocket error:', error);
        this.stateSubject.next(WebSocketState.Error);
      };
      
      this.socket.onclose = () => {
        console.log('WebSocket disconnected');
        this.stateSubject.next(WebSocketState.Disconnected);
        this.socket = null;
        
        // Attempt to reconnect
        if (this.reconnectAttempts < this.maxReconnectAttempts) {
          this.reconnectAttempts++;
          console.log(`Reconnecting... (attempt ${this.reconnectAttempts}/${this.maxReconnectAttempts})`);
          setTimeout(() => this.connect(token), this.reconnectDelay);
        }
      };
    } catch (error) {
      console.error('Failed to connect to WebSocket:', error);
      this.stateSubject.next(WebSocketState.Error);
    }
  }

  /**
   * Disconnect from WebSocket server
   */
  disconnect(): void {
    if (this.socket) {
      this.socket.close();
      this.socket = null;
      this.stateSubject.next(WebSocketState.Disconnected);
    }
  }

  /**
   * Get connection state as observable
   */
  getState(): Observable<WebSocketState> {
    return this.stateSubject.asObservable();
  }

  /**
   * Check if WebSocket is connected
   */
  isConnected(): boolean {
    return this.socket !== null && this.socket.readyState === WebSocket.OPEN;
  }

  /**
   * Subscribe to a room (sprint, project)
   * 
   * @param room Room identifier (e.g., 'sprint:sprint-123', 'project:project-456')
   * 
   * @example
   * ```typescript
   * this.wsService.joinRoom('sprint:sprint-123');
   * ```
   */
  joinRoom(room: string): void {
    if (this.isConnected()) {
      const message: WebSocketMessage = {
        type: WebSocketMessageType.JoinRoom,
        room: room,
        data: {},
        trace_id: this.generateTraceId(),
        timestamp: new Date().toISOString()
      };
      this.socket!.send(JSON.stringify(message));
      console.log(`Joined room: ${room}`);
    } else {
      console.warn('Cannot join room: WebSocket not connected');
    }
  }

  /**
   * Unsubscribe from a room
   * 
   * @param room Room identifier
   * 
   * @example
   * ```typescript
   * this.wsService.leaveRoom('sprint:sprint-123');
   * ```
   */
  leaveRoom(room: string): void {
    if (this.isConnected()) {
      const message: WebSocketMessage = {
        type: WebSocketMessageType.LeaveRoom,
        room: room,
        data: {},
        trace_id: this.generateTraceId(),
        timestamp: new Date().toISOString()
      };
      this.socket!.send(JSON.stringify(message));
      console.log(`Left room: ${room}`);
    }
  }

  /**
   * Get all WebSocket messages as observable
   */
  messages(): Observable<WebSocketMessage> {
    return this.messageSubject.asObservable();
  }

  /**
   * Subscribe to work item created events
   */
  onWorkItemCreated(): Observable<WorkItemEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.WorkItemCreated)
    ) as Observable<WorkItemEventData>;
  }

  /**
   * Subscribe to work item updated events
   */
  onWorkItemUpdated(): Observable<WorkItemEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.WorkItemUpdated)
    ) as Observable<WorkItemEventData>;
  }

  /**
   * Subscribe to work item deleted events
   */
  onWorkItemDeleted(): Observable<WorkItemEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.WorkItemDeleted)
    ) as Observable<WorkItemEventData>;
  }

  /**
   * Subscribe to work item status changed events
   */
  onWorkItemStatusChanged(): Observable<WorkItemEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.WorkItemStatusChanged)
    ) as Observable<WorkItemEventData>;
  }

  /**
   * Subscribe to sprint created events
   */
  onSprintCreated(): Observable<SprintEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.SprintCreated)
    ) as Observable<SprintEventData>;
  }

  /**
   * Subscribe to sprint updated events
   */
  onSprintUpdated(): Observable<SprintEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.SprintUpdated)
    ) as Observable<SprintEventData>;
  }

  /**
   * Subscribe to sprint closed events
   */
  onSprintClosed(): Observable<SprintEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.SprintClosed)
    ) as Observable<SprintEventData>;
  }

  /**
   * Subscribe to sprint status changed events
   */
  onSprintStatusChanged(): Observable<SprintEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.SprintStatusChanged)
    ) as Observable<SprintEventData>;
  }

  /**
   * Subscribe to comment added events
   */
  onCommentAdded(): Observable<CommentEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.CommentAdded)
    ) as Observable<CommentEventData>;
  }

  /**
   * Subscribe to comment updated events
   */
  onCommentUpdated(): Observable<CommentEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.CommentUpdated)
    ) as Observable<CommentEventData>;
  }

  /**
   * Subscribe to comment deleted events
   */
  onCommentDeleted(): Observable<CommentEventData> {
    return this.messages().pipe(
      filter(msg => msg.type === WebSocketMessageType.CommentDeleted)
    ) as Observable<CommentEventData>;
  }

  /**
   * Subscribe to messages for a specific room
   * 
   * @param room Room identifier
   */
  messagesForRoom(room: string): Observable<WebSocketMessage> {
    return this.messages().pipe(
      filter(msg => msg.room === room)
    );
  }

  /**
   * Generate a trace ID for correlation
   */
  private generateTraceId(): string {
    return 'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g, (c) => {
      const r = Math.random() * 16 | 0;
      const v = c === 'x' ? r : (r & 0x3 | 0x8);
      return v.toString(16);
    });
  }

  ngOnDestroy(): void {
    this.disconnect();
    this.messageSubject.complete();
    this.stateSubject.complete();
  }
}
