#!/bin/bash

# Debug script to check Keycloak user status

set -e

echo "🔍 Debugging Keycloak User Status"
echo ""

# Get admin token
echo "1️⃣  Getting admin token..."
ADMIN_TOKEN=$(curl -s -X POST "http://localhost:8080/realms/master/protocol/openid-connect/token" \
  -d "grant_type=password" \
  -d "client_id=admin-cli" \
  -d "username=admin" \
  -d "password=admin" | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)

if [ -z "$ADMIN_TOKEN" ]; then
    echo "❌ Failed to get admin token"
    exit 1
fi

echo "✅ Got admin token"
echo ""

# Get user details
echo "2️⃣  Getting user details for ttrpg_user..."
USER_DETAILS=$(curl -s -X GET "http://localhost:8080/admin/realms/ttrpg/users?username=ttrpg_user" \
  -H "Authorization: Bearer $ADMIN_TOKEN")

echo "User Details:"
echo "$USER_DETAILS" | jq '.'
echo ""

# Extract user ID
USER_ID=$(echo "$USER_DETAILS" | jq -r '.[0].id')

if [ -z "$USER_ID" ] || [ "$USER_ID" = "null" ]; then
    echo "❌ User not found"
    exit 1
fi

echo "User ID: $USER_ID"
echo ""

# Check if user is enabled
ENABLED=$(echo "$USER_DETAILS" | jq -r '.[0].enabled')
EMAIL_VERIFIED=$(echo "$USER_DETAILS" | jq -r '.[0].emailVerified')

echo "3️⃣  User Status:"
echo "  • Enabled: $ENABLED"
echo "  • Email Verified: $EMAIL_VERIFIED"
echo ""

# Get user credentials
echo "4️⃣  Checking credentials..."
CREDENTIALS=$(curl -s -X GET "http://localhost:8080/admin/realms/ttrpg/users/$USER_ID/credentials" \
  -H "Authorization: Bearer $ADMIN_TOKEN")

echo "Credentials:"
echo "$CREDENTIALS" | jq '.'
echo ""

if [ "$ENABLED" != "true" ]; then
    echo "⚠️  User is NOT enabled!"
    echo "   Fix: Enable the user in Keycloak admin console"
fi

if [ $(echo "$CREDENTIALS" | jq '. | length') -eq 0 ]; then
    echo "⚠️  No credentials found!"
    echo "   Fix: Set a password for the user in Keycloak admin console"
fi

echo ""
echo "💡 To fix the user manually:"
echo "   1. Go to http://localhost:8080"
echo "   2. Login as admin"
echo "   3. Select 'ttrpg' realm"
echo "   4. Go to Users → ttrpg_user"
echo "   5. Ensure 'Enabled' is ON"
echo "   6. Go to Credentials tab"
echo "   7. Click 'Set password' and set it to 'password'"
echo "   8. Make sure 'Temporary' is OFF"
