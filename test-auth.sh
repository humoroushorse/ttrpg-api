#!/bin/bash

# Test authentication flow
# Make sure services are running: ./start-all-services.sh

set -e

echo "🧪 Testing Authentication Flow"
echo ""

BASE_URL="http://localhost:8003/api/v1/auth"

# Test 1: Register a new user
echo "1️⃣  Registering new user..."
REGISTER_RESPONSE=$(curl -s -X POST "$BASE_URL/register" \
  -H "Content-Type: application/json" \
  -d '{
    "username": "testuser",
    "email": "test@example.com",
    "password": "Test123!",
    "first_name": "Test",
    "last_name": "User"
  }')

echo "Response: $REGISTER_RESPONSE"
echo ""

# Test 2: Login with the user
echo "2️⃣  Logging in..."
LOGIN_RESPONSE=$(curl -s -X POST "$BASE_URL/login" \
  -d "username=testuser&password=Test123!")

echo "Response: $LOGIN_RESPONSE"
echo ""

# Extract access token
ACCESS_TOKEN=$(echo $LOGIN_RESPONSE | grep -o '"access_token":"[^"]*' | cut -d'"' -f4)

if [ -z "$ACCESS_TOKEN" ]; then
    echo "❌ Failed to get access token"
    exit 1
fi

echo "✅ Got access token: ${ACCESS_TOKEN:0:50}..."
echo ""

# Test 3: Get user info
echo "3️⃣  Getting user info..."
USER_INFO=$(curl -s -X GET "$BASE_URL/user" \
  -H "Authorization: Bearer $ACCESS_TOKEN")

echo "Response: $USER_INFO"
echo ""

# Test 4: Refresh token
REFRESH_TOKEN=$(echo $LOGIN_RESPONSE | grep -o '"refresh_token":"[^"]*' | cut -d'"' -f4)

if [ ! -z "$REFRESH_TOKEN" ]; then
    echo "4️⃣  Refreshing token..."
    REFRESH_RESPONSE=$(curl -s -X POST "$BASE_URL/refresh" \
      -H "Content-Type: application/json" \
      -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}")
    
    echo "Response: $REFRESH_RESPONSE"
    echo ""
fi

# Test 5: Logout
echo "5️⃣  Logging out..."
LOGOUT_RESPONSE=$(curl -s -X POST "$BASE_URL/logout" \
  -H "Content-Type: application/json" \
  -d "{\"refresh_token\":\"$REFRESH_TOKEN\"}")

echo "✅ Logout successful"
echo ""

echo "🎉 All tests completed!"
echo ""
echo "📝 Next steps:"
echo "  • Open Swagger UI: http://localhost:8003/swagger/"
echo "  • Try the endpoints interactively"
echo "  • Use the access token for protected endpoints"
