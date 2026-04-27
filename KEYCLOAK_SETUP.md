# Keycloak Setup for TTRPG API

## Quick Setup

### 1. Access Keycloak Admin Console

Open: http://localhost:8080

Login with:
- Username: `admin`
- Password: `admin` (or whatever you set)

### 2. Create Realm

1. Click the dropdown in top-left corner (currently shows "master")
2. Click "Create Realm"
3. Enter realm name: `ttrpg`
4. Click "Create"

### 3. Create Client for Auth Service

1. In the `ttrpg` realm, go to "Clients" in the left menu
2. Click "Create client"
3. Fill in:
   - Client ID: `auth-service`
   - Client type: `OpenID Connect`
   - Click "Next"
4. Configure:
   - Client authentication: `ON` (if you want client secret)
   - Authorization: `OFF`
   - Authentication flow: Check "Standard flow" and "Direct access grants"
   - Click "Save"

### 4. Get Client Secret (if using)

1. Go to the "Credentials" tab
2. Copy the "Client secret"
3. Set environment variable:
   ```bash
   export KEYCLOAK_CLIENT_SECRET=your-secret-here
   ```

### 5. Configure Service Account (for user creation)

The auth service needs admin access to create users.

**Option A: Use admin credentials (development only)**
```bash
export KEYCLOAK_ADMIN_USER=admin
export KEYCLOAK_ADMIN_PASS=admin
```

**Option B: Create service account (recommended for production)**
1. In the client settings, enable "Service accounts roles"
2. Go to "Service account roles" tab
3. Assign role: `realm-admin` or `manage-users`

## Testing

Once configured, test the auth service:

```bash
# Register a user
curl -X POST http://localhost:8082/api/v1/auth/register \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "Test123!",
    "first_name": "Test",
    "last_name": "User"
  }'

# Login
curl -X POST http://localhost:8082/api/v1/auth/login \
  -d "username=testuser&password=Test123!"
```

## Environment Variables

The auth service uses these Keycloak settings:

| Variable | Default | Description |
|----------|---------|-------------|
| `KEYCLOAK_URL` | http://localhost:8080 | Keycloak server URL |
| `KEYCLOAK_REALM` | ttrpg | Realm name |
| `KEYCLOAK_CLIENT_ID` | auth-service | Client ID |
| `KEYCLOAK_CLIENT_SECRET` | (empty) | Client secret (optional) |
| `KEYCLOAK_ADMIN_USER` | admin | Admin username for user creation |
| `KEYCLOAK_ADMIN_PASS` | admin | Admin password for user creation |

## Troubleshooting

### "Realm not found"
- Make sure you created the `ttrpg` realm in Keycloak
- Or set `KEYCLOAK_REALM` to match your existing realm

### "Invalid client credentials"
- Check that the client ID matches: `auth-service`
- If using client secret, make sure it's set correctly

### "Unauthorized" when creating users
- Check admin credentials are correct
- Make sure admin user has permission to create users in the realm

## Using with py_dnd

If you want to use the same Keycloak setup as py_dnd:

1. Find out what realm py_dnd uses
2. Set the environment variable:
   ```bash
   export KEYCLOAK_REALM=your-pydnd-realm
   ```
3. Restart the auth service

## Production Notes

For production:
- Use strong admin passwords
- Enable client authentication with secrets
- Use service accounts instead of admin credentials
- Enable HTTPS
- Configure proper CORS settings
- Set up realm-specific roles and permissions
