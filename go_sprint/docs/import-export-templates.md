# Import/Export Templates and Documentation

## Overview

The Sprint Management System supports importing and exporting work items and sprints in multiple formats: CSV, JSON, and Excel. This document provides templates and guidelines for data import/export operations.

## Work Items Import/Export

### CSV Template for Work Items

```csv
id,type,title,description,status,priority,story_points,assignee_id,reporter_id,parent_id,sprint_id
,epic,User Authentication,Implement complete user authentication system,todo,high,13,a1b2c3d4-e5f6-7890-abcd-ef1234567890,b2c3d4e5-f6a7-8901-bcde-f12345678901,,
,story,Login Page,Create login page with email and password,in_progress,medium,5,a1b2c3d4-e5f6-7890-abcd-ef1234567890,b2c3d4e5-f6a7-8901-bcde-f12345678901,<epic-id>,c3d4e5f6-a7b8-9012-cdef-123456789012
,defect,Login Button Not Working,Fix login button click handler,todo,critical,3,a1b2c3d4-e5f6-7890-abcd-ef1234567890,b2c3d4e5-f6a7-8901-bcde-f12345678901,<story-id>,c3d4e5f6-a7b8-9012-cdef-123456789012
```

### JSON Template for Work Items

```json
[
  {
    "id": "",
    "type": "epic",
    "title": "User Authentication",
    "description": "Implement complete user authentication system",
    "status": "todo",
    "priority": "high",
    "story_points": "13",
    "assignee_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "reporter_id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
    "parent_id": "",
    "sprint_id": ""
  },
  {
    "id": "",
    "type": "story",
    "title": "Login Page",
    "description": "Create login page with email and password",
    "status": "in_progress",
    "priority": "medium",
    "story_points": "5",
    "assignee_id": "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
    "reporter_id": "b2c3d4e5-f6a7-8901-bcde-f12345678901",
    "parent_id": "<epic-id>",
    "sprint_id": "c3d4e5f6-a7b8-9012-cdef-123456789012"
  }
]
```

## Sprints Import/Export

### CSV Template for Sprints

```csv
id,name,description,status,start_date,end_date,capacity_points,committed_points,completed_points,created_by
,Sprint 1,First sprint of the project,active,2024-01-01,2024-01-14,50,45,30,b2c3d4e5-f6a7-8901-bcde-f12345678901
,Sprint 2,Second sprint of the project,planned,2024-01-15,2024-01-28,50,0,0,b2c3d4e5-f6a7-8901-bcde-f12345678901
```

### JSON Template for Sprints

```json
[
  {
    "id": "",
    "name": "Sprint 1",
    "description": "First sprint of the project",
    "status": "active",
    "start_date": "2024-01-01",
    "end_date": "2024-01-14",
    "capacity_points": "50",
    "committed_points": "45",
    "completed_points": "30",
    "created_by": "b2c3d4e5-f6a7-8901-bcde-f12345678901"
  }
]
```

## Field Descriptions

### Work Item Fields

| Field | Required | Type | Description | Valid Values |
|-------|----------|------|-------------|--------------|
| id | No | UUID | Unique identifier (leave empty for new items) | Valid UUID or empty |
| type | Yes | String | Type of work item | epic, story, defect |
| title | Yes | String | Short title of the work item | Any non-empty string |
| description | Yes | String | Detailed description | Any non-empty string |
| status | No | String | Current status (defaults to 'todo') | todo, in_progress, in_review, done, blocked |
| priority | Yes | String | Priority level | low, medium, high, critical |
| story_points | No | Integer | Estimation in story points | Positive integer |
| assignee_id | No | UUID | ID of assigned user | Valid UUID or empty |
| reporter_id | Yes | UUID | ID of user who created the item | Valid UUID |
| parent_id | No | UUID | ID of parent work item | Valid UUID or empty |
| sprint_id | No | UUID | ID of associated sprint | Valid UUID or empty |

### Sprint Fields

| Field | Required | Type | Description | Valid Values |
|-------|----------|------|-------------|--------------|
| id | No | UUID | Unique identifier (leave empty for new sprints) | Valid UUID or empty |
| name | Yes | String | Sprint name | Any non-empty string |
| description | No | String | Sprint description | Any string |
| status | No | String | Sprint status (defaults to 'planned') | planned, active, completed, cancelled |
| start_date | Yes | Date | Sprint start date | YYYY-MM-DD format |
| end_date | Yes | Date | Sprint end date | YYYY-MM-DD format (must be after start_date) |
| capacity_points | No | Integer | Team capacity in story points | Positive integer |
| committed_points | No | Integer | Committed story points | Non-negative integer |
| completed_points | No | Integer | Completed story points | Non-negative integer |
| created_by | Yes | UUID | ID of user who created the sprint | Valid UUID |

## Import Conflict Resolution Strategies

When importing data, you can specify how to handle conflicts (existing records with the same ID):

### Skip Strategy
- **Behavior**: Skips records that already exist
- **Use Case**: When you want to preserve existing data and only add new records
- **Example**: Initial data migration where you don't want to overwrite existing work

### Update Strategy
- **Behavior**: Updates existing records with new data from import
- **Use Case**: When you want to sync data from external systems
- **Example**: Regular data synchronization from another project management tool

### Error Strategy
- **Behavior**: Returns an error when a conflict is detected
- **Use Case**: When you want strict control and need to manually resolve conflicts
- **Example**: Critical data imports where you need to review each conflict

## Import Validation

The import process validates all records before importing:

1. **Required Fields**: All required fields must be present and non-empty
2. **Data Types**: Fields must match expected types (UUID, integer, date, etc.)
3. **Valid Values**: Enum fields (type, status, priority) must have valid values
4. **Relationships**: Parent-child relationships must be valid (e.g., stories can only have epic parents)
5. **Date Validation**: Dates must be in correct format and logical (end_date > start_date)

## Error Reporting

Import operations return detailed error reports:

```json
{
  "total_records": 100,
  "success_count": 95,
  "skipped_count": 3,
  "error_count": 2,
  "errors": [
    {
      "row": 5,
      "field": "type",
      "message": "invalid type: task (must be epic, story, or defect)",
      "value": "task"
    },
    {
      "row": 12,
      "message": "validation failed: reporter_id is required"
    }
  ]
}
```

## Export Filtering

When exporting data, you can apply filters to export only specific records:

### Work Item Filters
- **Type**: Filter by work item type (epic, story, defect)
- **Status**: Filter by status (todo, in_progress, etc.)
- **Sprint**: Filter by sprint ID
- **Assignee**: Filter by assignee ID
- **Date Range**: Filter by creation or update date

### Sprint Filters
- **Status**: Filter by sprint status
- **Date Range**: Filter by start or end date

## Best Practices

### Importing Data

1. **Start Small**: Test with a small dataset first to validate your data format
2. **Use IDs Carefully**: Leave ID field empty for new records, provide ID only for updates
3. **Validate Externally**: Validate your data before importing to catch errors early
4. **Use Skip Strategy**: For initial imports, use skip strategy to avoid overwriting existing data
5. **Check Error Reports**: Always review error reports after import to identify issues

### Exporting Data

1. **Choose Right Format**: 
   - CSV: For simple data analysis and spreadsheet tools
   - JSON: For programmatic processing and API integration
   - Excel: For business users and formatted reports
2. **Apply Filters**: Export only the data you need to reduce file size
3. **Regular Backups**: Schedule regular exports for backup purposes
4. **Version Control**: Keep track of export timestamps for data versioning

## API Endpoints

### Import Endpoints

```
POST /api/v1/import/workitems/csv
POST /api/v1/import/workitems/json
POST /api/v1/import/sprints/csv
POST /api/v1/import/sprints/json
```

Query Parameters:
- `strategy`: Conflict resolution strategy (skip, update, error)

### Export Endpoints

```
GET /api/v1/export/workitems/csv
GET /api/v1/export/workitems/json
GET /api/v1/export/workitems/excel
GET /api/v1/export/sprints/csv
GET /api/v1/export/sprints/json
GET /api/v1/export/sprints/excel
```

Query Parameters:
- `type`: Work item type filter
- `status`: Status filter
- `sprint_id`: Sprint ID filter
- `start_date`: Start date filter (YYYY-MM-DD)
- `end_date`: End date filter (YYYY-MM-DD)

## Examples

### Example 1: Import Work Items from CSV

```bash
curl -X POST http://localhost:8080/api/v1/import/workitems/csv?strategy=skip \
  -H "Content-Type: text/csv" \
  --data-binary @workitems.csv
```

### Example 2: Export Work Items to Excel

```bash
curl -X GET "http://localhost:8080/api/v1/export/workitems/excel?status=in_progress" \
  -o workitems.xlsx
```

### Example 3: Import Sprints from JSON with Update Strategy

```bash
curl -X POST http://localhost:8080/api/v1/import/sprints/json?strategy=update \
  -H "Content-Type: application/json" \
  -d @sprints.json
```

## Troubleshooting

### Common Import Errors

1. **"invalid type: X"**: The type field must be one of: epic, story, defect
2. **"invalid UUID format"**: UUIDs must be in format: xxxxxxxx-xxxx-xxxx-xxxx-xxxxxxxxxxxx
3. **"reporter_id is required"**: Every work item must have a reporter_id
4. **"end_date must be after start_date"**: Sprint dates must be in logical order
5. **"invalid parent-child relationship"**: Check parent-child type rules (stories can only have epic parents)

### Performance Considerations

- **Large Imports**: For imports over 1000 records, consider splitting into smaller batches
- **Validation Time**: Validation adds overhead; use skip strategy for faster imports when data is pre-validated
- **Excel Exports**: Excel format is slower than CSV/JSON for large datasets
- **Memory Usage**: Large exports may require significant memory; use pagination for very large datasets
