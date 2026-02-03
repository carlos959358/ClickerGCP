# Consumer Service Logging Guide

The consumer service now includes comprehensive logging to help diagnose errors and trace message processing flow. Every operation logs its status, making it easy to identify where problems occur.

## Log Format and Tags

All logs use consistent tags for easy filtering and scanning:

```
[TAG] Log message
```

Common tags:
- `[Services]` - Service initialization
- `[Auth]` - Authentication validation
- `[/process]` - Main Pub/Sub message processing endpoint
- `[/health]` - Health check endpoint
- `[/live]` - Liveness probe endpoint
- `[Server]` - HTTP server startup/shutdown
- `[Firestore]` - Firestore database operations
- `[Notifier]` - Backend notification (HTTP calls)

## Message Processing Flow

When a Pub/Sub message arrives at `/process`, you'll see logs in this order:

### 1. Request Received
```
[/process] ===== START =====
```

### 2. Authentication
```
[Auth] Token validation warning (may be running locally): <error>
```
- Info only; doesn't block processing

### 3. Payload Parsing
```
[/process] ✓ Raw payload decoded: <payload>
[/process] ✓ Message is map with keys: [message]
[/process] ✓ Message ID: msg-123456
```
- If any step fails, error message shows what went wrong

### 4. Idempotency Check
```
[Firestore] CheckIdempotency: Checking if messageID=msg-123456 was already processed
[Firestore] ✓ Message msg-123456 not in processed_messages (new message)
```
- Or if already processed:
```
[Firestore] WARN: Message msg-123456 already processed (idempotent)
[/process] ✓ Message msg-123456 already processed (idempotent, returning 200)
```

### 5. Data Extraction and Decoding
```
[/process] ✓ Data field found, length: 150 bytes
[/process] ✓ Base64 decoded, result: {"timestamp":1706000000,"country":"US","ip":"1.2.3.4"}
[/process] ✓ Event parsed: Country=US, IP=1.2.3.4, Timestamp=1706000000
```

### 6. Counter Update (Firestore Transaction)
```
[Firestore] IncrementCounters: country=US, code=US
[Firestore] Transaction started for country=US
[Firestore] Updating global counter at path: counters/global
[Firestore] ✓ Global counter incremented
[Firestore] Updating country counter at path: counters/country_US
[Firestore] ✓ Country counter incremented for country_US
[Firestore] ✓ IncrementCounters completed successfully for country=US
[/process] ✓ Counters incremented for country: US
```

### 7. Record Processed Message
```
[Firestore] RecordProcessedMessage: Recording messageID=msg-123456, country=US
[Firestore] ✓ Processed message recorded: msg-123456
[/process] ✓ Message msg-123456 recorded as processed
```

### 8. Retrieve Updated Counters
```
[Firestore] GetCounters: Starting to fetch all counters
[Firestore] Fetching global counter from counters/global
[Firestore] ✓ Global counter retrieved: 42
[Firestore] Fetching all country counters from counters collection
[Firestore] Retrieved 5 documents from counters collection
[Firestore] Country counter: country_US = 15 (name: US)
[Firestore] Country counter: country_GB = 8 (name: GB)
[Firestore] Country counter: country_DE = 10 (name: DE)
[Firestore] ✓ GetCounters completed: 3 countries found
[/process] ✓ Counters retrieved: map[...]
```

### 9. Notify Backend (Best-Effort)
```
[Notifier] NotifyCounterUpdate: global=42, countries=3
[Notifier] Marshaling payload to JSON
[Notifier] ✓ Payload marshaled, size: 456 bytes
[Notifier] POSTing to URL: https://clicker-backend.example.com/internal/broadcast
[Notifier] ✓ Response received with status: 200 OK
[Notifier] ✓ Backend notification successful
[/process] ✓ Backend notified successfully
```

### 10. Success Response
```
[/process] ===== SUCCESS =====
```

## Error Scenarios and What to Look For

### Scenario 1: Message Never Reaches `/process`

**What to check:**
1. Check Cloud Run logs for the consumer service
2. Check Cloud Logging for Pub/Sub push delivery logs
3. Verify consumer service URL is correct in Pub/Sub subscription
4. Check that Pub/Sub push endpoint is configured correctly

```bash
# Check consumer service URL
gcloud run services describe clicker-consumer --region=europe-southwest1 --format='value(status.url)'

# Check subscription configuration
gcloud pubsub subscriptions describe click-events-sub --format='yaml(pushConfig)'
```

### Scenario 2: Request Body Parse Error

**Log signature:**
```
[/process] ERROR: JSON decode failed: <error>
```

**Possible causes:**
- Pub/Sub message format is invalid
- Payload is not valid JSON
- Request body is corrupted

**Check:**
- Pub/Sub topic and subscription configuration
- Message format from the backend

### Scenario 3: Missing Data Field

**Log signature:**
```
[/process] ERROR: No 'data' field or not string
[/process] ERROR: Base64 decode failed: <error>
```

**Possible causes:**
- Backend not publishing proper Pub/Sub message format
- Pub/Sub message data is not properly base64 encoded

**Check:**
```bash
# Manually publish test message to Pub/Sub
gcloud pubsub topics publish click-events --message='{"timestamp":1706000000,"country":"TEST","ip":"1.2.3.4"}'
```

### Scenario 4: Firestore Connection Error

**Log signature:**
```
[Firestore] ERROR: Failed to create client: <error>
[Firestore] ERROR: IncrementCounters transaction failed: <error>
[Firestore] ERROR: Failed to check idempotency: <error>
```

**Possible causes:**
- Firestore not accessible (API disabled, network issue)
- Service account doesn't have Firestore permissions
- Database ID incorrect or database not created
- Firestore quota exceeded

**Check:**
```bash
# Verify Firestore database exists
gcloud firestore databases list

# Verify service account has proper IAM roles
gcloud projects get-iam-policy dev-trail-475809-v2 \
  --flatten="bindings[].members" \
  --filter="bindings.members:clicker-consumer@*" \
  --format="table(bindings.role)"
```

### Scenario 5: Backend Notification Fails

**Log signature:**
```
[Notifier] ERROR: Failed to POST to backend: <error>
[Notifier] ERROR: Backend returned non-OK status 500 with body: ...
```

**Possible causes:**
- Backend service is down or unreachable
- Backend `/internal/broadcast` endpoint error
- Network connectivity issue between consumer and backend
- Backend URL configured incorrectly

**Check:**
```bash
# Verify backend is running
BACKEND_URL=$(gcloud run services describe clicker-backend --region=europe-southwest1 --format='value(status.url)')
curl -s "$BACKEND_URL/health" | jq .

# Test the broadcast endpoint directly
curl -X POST "$BACKEND_URL/internal/broadcast" \
  -H "Content-Type: application/json" \
  -d '{"type":"test","global":1,"countries":{}}'
```

## Filtering Logs by Tag

### View only Firestore operations:
```bash
gcloud run services logs read clicker-consumer --region=europe-southwest1 | grep "\[Firestore\]"
```

### View only errors:
```bash
gcloud run services logs read clicker-consumer --region=europe-southwest1 | grep "ERROR"
```

### View only successful operations:
```bash
gcloud run services logs read clicker-consumer --region=europe-southwest1 | grep "✓"
```

### View full processing flow for a specific message:
```bash
gcloud run services logs read clicker-consumer --region=europe-southwest1 | grep "msg-123456"
```

## Log Levels

- **✓ Success**: Operation completed successfully
- **WARN**: Non-fatal warning, operation continues
- **ERROR**: Operation failed, error returned to client
- **INFO**: Informational message, normal operation

## Quick Diagnostic Checklist

When something isn't working:

1. ✅ **Check if `/process` endpoint is being called:**
   ```
   grep "\[/process\]" consumer-logs.txt | head -5
   ```

2. ✅ **Check for Firestore errors:**
   ```
   grep "\[Firestore\] ERROR" consumer-logs.txt
   ```

3. ✅ **Check for notification errors:**
   ```
   grep "\[Notifier\] ERROR" consumer-logs.txt
   ```

4. ✅ **Check full message processing flow:**
   ```
   grep "===== START =====" consumer-logs.txt  # Where messages start
   grep "===== SUCCESS =====" consumer-logs.txt  # Where messages succeed
   ```

5. ✅ **Check service initialization:**
   ```
   grep "\[Services\]" consumer-logs.txt
   ```

## Sample Complete Success Flow

Here's what a complete successful message processing looks like:

```
[/process] ===== START =====
[/process] ✓ Raw payload decoded: map[message:map[data:eyJ0aW1lc3RhbXAiOjE3MDYwMDAwMDAsImNvdW50cnkiOiJVUyIsImlwIjoiMS4yLjMuNCJ9 messageId:123456]]
[/process] ✓ Message is map with keys: [message]
[/process] ✓ Message ID: 123456
[Firestore] CheckIdempotency: Checking if messageID=123456 was already processed
[Firestore] ✓ Message 123456 not in processed_messages (new message)
[/process] ✓ Data field found, length: 64 bytes
[/process] ✓ Base64 decoded, result: {"timestamp":1706000000,"country":"US","ip":"1.2.3.4"}
[/process] ✓ Event parsed: Country=US, IP=1.2.3.4, Timestamp=1706000000
[/process] ✓ Updater initialized
[Firestore] IncrementCounters: country=US, code=US
[Firestore] Transaction started for country=US
[Firestore] Updating global counter at path: counters/global
[Firestore] ✓ Global counter incremented
[Firestore] Updating country counter at path: counters/country_US
[Firestore] ✓ Country counter incremented for country_US
[Firestore] ✓ IncrementCounters completed successfully for country=US
[/process] ✓ Counters incremented for country: US
[Firestore] RecordProcessedMessage: Recording messageID=123456, country=US
[Firestore] ✓ Processed message recorded: 123456
[/process] ✓ Message 123456 recorded as processed
[Firestore] GetCounters: Starting to fetch all counters
[Firestore] Fetching global counter from counters/global
[Firestore] ✓ Global counter retrieved: 1
[Firestore] Fetching all country counters from counters collection
[Firestore] Retrieved 1 documents from counters collection
[Firestore] Country counter: country_US = 1 (name: US)
[Firestore] ✓ GetCounters completed: 1 countries found
[/process] ✓ Counters retrieved: map[countries:map[country_US:map[count:1 country:US]] global:1]
[Notifier] NotifyCounterUpdate: global=1, countries=1
[Notifier] Marshaling payload to JSON
[Notifier] ✓ Payload marshaled, size: 123 bytes
[Notifier] POSTing to URL: https://clicker-backend.example.com/internal/broadcast
[Notifier] ✓ Response received with status: 200 OK
[Notifier] ✓ Backend notification successful
[/process] ✓ Backend notified successfully
[/process] ===== SUCCESS =====
```

## Real-Time Log Monitoring

To monitor logs in real-time as messages are processed:

```bash
# Watch consumer logs in real-time
gcloud run services logs read clicker-consumer --region=europe-southwest1 --follow

# Or filter to specific operation
gcloud run services logs read clicker-consumer --region=europe-southwest1 --follow | grep -E "\[Firestore\]|\[Notifier\]"
```

## Accessing Logs from Cloud Console

1. Go to Cloud Run > clicker-consumer service
2. Click "Logs" tab
3. Logs appear in JSON format with timestamps
4. Use search/filter features to find specific messages

The structured logging with tags makes it easy to find and understand what's happening in the service!
