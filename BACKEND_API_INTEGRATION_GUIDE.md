# Backend API Integration Guide

**Complete guide for integrating the Sprint Management API with Angular frontend applications**

---

## Table of Contents

1. [Overview](#overview)
2. [Quick Start](#quick-start)
3. [Authentication with Keycloak](#authentication-with-keycloak)
4. [TypeScript Interfaces](#typescript-interfaces)
5. [Angular Services](#angular-services)
6. [WebSocket Integration](#websocket-integration)
7. [Component Examples](#component-examples)
8. [Error Handling](#error-handling)
9. [Environment Configuration](#environment-configuration)
10. [Complete Setup Guide](#complete-setup-guide)
11. [Best Practices](#best-practices)
12. [Troubleshooting](#troubleshooting)

---

## Overview

The Sprint Management API provides RESTful endpoints for managing work items, sprints, and team collaboration. This guide contains everything you need to integrate with the backend API.

### API Information

- **Base URL (Development)**: `http://localhost:8082`
- **Base URL (Production)**: `https://api.sprint.example.com`
- **API Version**: `v1`
- **All endpoints prefixed with**: `/api/v1`
- **Authentication**: JWT tokens via Keycloak
- **Real-time Updates**: WebSocket support at `/ws`

### Available Endpoints

**Authentication** (5 endpoints):
- POST `/api/v1/auth/login` - User login
- POST `/api/v1/auth/refresh` - Refresh access token
- POST `/api/v1/auth/logout` - User logout
- GET `/api/v1/auth/user` - Get current user
- POST `/api/v1/auth/register` - Register new user

**Work Items** (10 operations):
- GET `/api/v1/workitems` - List work items
- POST `/api/v1/workitems` - Create work item
- GET `/api/v1/workitems/{id}` - Get work item
- PUT `/api/v1/workitems/{id}` - Update work item
- DELETE `/api/v1/workitems/{id}` - Delete work item
- POST `/api/v1/workitems/search` - Search work items
- GET `/api/v1/workitems/{id}/dependencies` - Get dependencies
- POST `/api/v1/workitems/{id}/dependencies` - Create dependency
- DELETE `/api/v1/workitems/{id}/dependencies/{depId}` - Delete dependency
- GET `/api/v1/workitems/{id}/children` - Get child work items

**Sprints** (12 operations):
- GET `/api/v1/sprints` - List sprints
- POST `/api/v1/sprints` - Create sprint
