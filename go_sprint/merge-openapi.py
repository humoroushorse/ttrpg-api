#!/usr/bin/env python3
"""Merge OpenAPI specs into combined specs (with and without auth endpoints)."""

import yaml
import os
from pathlib import Path

# Get the directory where this script is located
script_dir = Path(__file__).parent.absolute()

# Read the individual spec files
with open(script_dir / 'api/openapi/workitems.yaml', 'r') as f:
    workitems = yaml.safe_load(f)

with open(script_dir / 'api/openapi/sprints.yaml', 'r') as f:
    sprints = yaml.safe_load(f)

with open(script_dir.parent / 'go_auth/api/openapi/auth.yaml', 'r') as f:
    auth = yaml.safe_load(f)

# Create combined spec WITHOUT auth
combined_no_auth = {
    'openapi': '3.0.3',
    'info': {
        'title': 'Sprint Management API',
        'version': '1.0.0',
        'description': 'Sprint and work item management API'
    },
    'servers': [
        {'url': 'http://localhost:8082', 'description': 'Local development server'}
    ],
    'paths': {},
    'components': {
        'schemas': {},
        'responses': {},
        'securitySchemes': {}
    }
}

# Merge paths from workitems and sprints
combined_no_auth['paths'].update(workitems.get('paths', {}))
combined_no_auth['paths'].update(sprints.get('paths', {}))

# Merge components from workitems and sprints
for spec in [workitems, sprints]:
    if 'components' in spec:
        if 'schemas' in spec['components']:
            combined_no_auth['components']['schemas'].update(spec['components']['schemas'])
        if 'responses' in spec['components']:
            combined_no_auth['components']['responses'].update(spec['components']['responses'])
        if 'securitySchemes' in spec['components']:
            combined_no_auth['components']['securitySchemes'].update(spec['components']['securitySchemes'])

# Add global security requirement
combined_no_auth['security'] = [{'BearerAuth': []}]

# Write combined spec WITHOUT auth
with open(script_dir / 'api/openapi/combined.yaml', 'w') as f:
    yaml.dump(combined_no_auth, f, default_flow_style=False, sort_keys=False)

print('✅ Combined spec (without auth) generated: api/openapi/combined.yaml')
print(f'📊 Total paths: {len(combined_no_auth["paths"])}')

# Create combined spec WITH auth
combined_with_auth = {
    'openapi': '3.0.3',
    'info': {
        'title': 'Sprint Management API with Authentication',
        'version': '1.0.0',
        'description': 'Complete API for sprint management including authentication'
    },
    'servers': [
        {'url': 'http://localhost:8082', 'description': 'Local development server'}
    ],
    'paths': {},
    'components': {
        'schemas': {},
        'responses': {},
        'securitySchemes': {}
    }
}

# Merge paths from workitems and sprints
combined_with_auth['paths'].update(workitems.get('paths', {}))
combined_with_auth['paths'].update(sprints.get('paths', {}))

# Add auth paths with /api/v1 prefix
for path, methods in auth.get('paths', {}).items():
    new_path = '/api/v1' + path
    combined_with_auth['paths'][new_path] = methods

# Merge components from all specs
for spec in [workitems, sprints, auth]:
    if 'components' in spec:
        if 'schemas' in spec['components']:
            combined_with_auth['components']['schemas'].update(spec['components']['schemas'])
        if 'responses' in spec['components']:
            combined_with_auth['components']['responses'].update(spec['components']['responses'])
        if 'securitySchemes' in spec['components']:
            combined_with_auth['components']['securitySchemes'].update(spec['components']['securitySchemes'])

# Add global security requirement
combined_with_auth['security'] = [{'BearerAuth': []}]

# Write combined spec WITH auth
with open(script_dir / 'api/openapi/combined-with-auth.yaml', 'w') as f:
    yaml.dump(combined_with_auth, f, default_flow_style=False, sort_keys=False)

print('✅ Combined spec (with auth) generated: api/openapi/combined-with-auth.yaml')
print(f'📊 Total paths: {len(combined_with_auth["paths"])}')
print('\nAll endpoints included:')
for path in sorted(combined_with_auth['paths'].keys()):
    print(f'  - {path}')
