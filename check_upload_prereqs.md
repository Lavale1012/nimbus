# Debugging File Upload 400 Error

## The Problem
You're getting a 400 Bad Request error when uploading files. This means the server is rejecting your request during validation.

## Most Likely Causes

### 1. User doesn't exist
Your CLI is hardcoded with:
```go
id := 1  // User ID 1
```

**Check:** Does a user with ID=1 exist in your database?

### 2. Box doesn't exist
Your CLI is hardcoded with:
```go
box_id := 8664386071129054956  // This specific box ID
```

**Check:** Does a box with box_id=8664386071129054956 exist in your database?

### 3. Box doesn't belong to user
Even if both exist, the box must belong to that specific user.

## How to Fix

### Option A: Create the test data first

1. **Register a user** (this creates a Home-Box automatically):
```bash
curl -X POST http://localhost:8080/register \
  -H "Content-Type: application/json" \
  -d '{
    "email": "test@example.com",
    "password": "Test123!@#",
    "passkey": "1234"
  }'
```

2. **Note the response** - it should include:
   - `user_id` - The ID of the created user
   - The user's Boxes array with a Home-Box that has a `box_id`

3. **Update your CLI** with the real IDs from step 2:
```go
// In client/cli/cmd/filePost.go
id := <USER_ID_FROM_RESPONSE>        // Replace with actual user_id
box_id := <BOX_ID_FROM_RESPONSE>     // Replace with actual box_id from Home-Box
```

### Option B: Add debug logging to see the exact error

Run the CLI again with the updated error display:
```bash
cd /Users/lavalebutterfield/Desktop/nim-cli/client
./nimbus post -f newtest.txt
```

The improved error message will now show you EXACTLY what the server says is wrong.

## Quick Test

Run this to create a test user:
```bash
cd /Users/lavalebutterfield/Desktop/nim-cli/server
./test_user_box.sh
```

## Common Error Messages You Might See

1. **"user_id is required"** - Form field not sent correctly
2. **"Invalid user_id"** - user_id is not a valid number
3. **"User with ID X not found"** - User doesn't exist in database
4. **"Box with ID X not found or does not belong to user Y"** - Box doesn't exist or wrong user
5. **"File size must be greater than zero"** - Empty file

## Debugging Steps

1. Make sure your server is running: `go run main.go`
2. Check server logs when you upload - they'll show the actual error
3. Run the upload again with the updated CLI to see the server response
4. Use the test script to create valid test data
