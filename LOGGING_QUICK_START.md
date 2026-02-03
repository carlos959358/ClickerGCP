# Consumer Service Logging - Quick Start

## What's New

The consumer service now has **comprehensive logging** at every step to diagnose errors:

✅ Firestore operations (connection, transactions, queries)
✅ HTTP notifications to backend
✅ Message processing flow
✅ Error details with full context

## View Logs in Real-Time

```bash
# Watch all consumer logs live
gcloud run services logs read clicker-consumer --region=europe-southwest1 --follow

# Watch only Firestore operations
gcloud run services logs read clicker-consumer --region=europe-southwest1 --follow | grep "\[Firestore\]"

# Watch only errors
gcloud run services logs read clicker-consumer --region=europe-southwest1 --follow | grep "ERROR"

# Watch message processing
gcloud run services logs read clicker-consumer --region=europe-southwest1 --follow | grep "\[/process\]"
```

## Log Format

Every log message has a **tag** in brackets:
```
[TAG] Message text
```

**Common tags:**
- `[/process]` - Message processing endpoint (main flow)
- `[Firestore]` - Database operations
- `[Notifier]` - Backend HTTP calls
- `[Server]` - HTTP server events
- `[Auth]` - Authentication checks

## Quick Diagnostic Checklist

When something isn't working:

1. **Is the message reaching the consumer?**
   ```bash
   gcloud run services logs read clicker-consumer --region=europe-southwest1 | grep "===== START =====" | tail -5
   ```

2. **Where is it failing?**
   ```bash
   gcloud run services logs read clicker-consumer --region=europe-southwest1 | grep "ERROR"
   ```

3. **Is Firestore working?**
   ```bash
   gcloud run services logs read clicker-consumer --region=europe-southwest1 | grep "\[Firestore\] ERROR"
   ```

4. **Is backend notification working?**
   ```bash
   gcloud run services logs read clicker-consumer --region=europe-southwest1 | grep "\[Notifier\]"
   ```

## Example Success Flow

You should see logs like this when everything works:

```
[/process] ===== START =====
[/process] ✓ Raw payload decoded: ...
[/process] ✓ Message ID: msg-123456
[Firestore] ✓ Message not in processed_messages (new message)
[Firestore] ✓ Global counter incremented
[Firestore] ✓ Country counter incremented for country_US
[/process] ✓ Counters incremented for country: US
[Notifier] ✓ Backend notification successful
[/process] ===== SUCCESS =====
```

## Example Error Flow

If something fails, you'll see:

```
[/process] ===== START =====
[/process] ✓ Raw payload decoded: ...
[Firestore] ERROR: Failed to create client: <error details>
[/process] ERROR: Service not ready
```

The error message tells you exactly what went wrong and where.

## Full Documentation

See **CONSUMER_LOGGING.md** for:
- Detailed message processing flow
- All error scenarios and how to fix them
- Log filtering examples
- Real-time monitoring instructions

## Enhanced Code Files

The following files now have comprehensive logging:

1. **consumer/firestore.go**
   - Firestore client initialization
   - Counter increment transactions
   - Database queries
   - Idempotency checks
   - Error details

2. **consumer/notifier.go**
   - HTTP payload marshaling
   - POST request details
   - Response status and body
   - HTTP errors

3. **consumer/main.go**
   - Endpoint request logging
   - Health check details
   - Server startup/shutdown
   - Service initialization

## Next Steps

1. Rebuild images (already done):
   ```bash
   gcloud builds submit --config=consumer/cloudbuild.yaml consumer/
   gcloud builds submit --config=backend/cloudbuild.yaml backend/
   ```

2. Test with a message:
   ```bash
   BACKEND_URL=$(gcloud run services describe clicker-backend --region=europe-southwest1 --format='value(status.url)')
   curl "$BACKEND_URL/click?country=TEST&ip=1.2.3.4"
   ```

3. Watch logs in real-time:
   ```bash
   gcloud run services logs read clicker-consumer --region=europe-southwest1 --follow | grep -E "\[/process\]|\[Firestore\]|\[Notifier\]|ERROR"
   ```

4. Check if counter incremented:
   ```bash
   curl "$BACKEND_URL/count" | jq .
   ```

This comprehensive logging will make it much easier to identify exactly where any errors occur in the message processing pipeline!
