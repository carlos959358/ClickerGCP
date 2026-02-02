#!/bin/bash

# Pub/Sub Message Flow Diagnostic Script
# This script checks all components of the message flow

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Counters
PASSED=0
FAILED=0
WARNINGS=0

# Helper functions
check_pass() {
    echo -e "${GREEN}✓${NC} $1"
    ((PASSED++))
}

check_fail() {
    echo -e "${RED}✗${NC} $1"
    ((FAILED++))
}

check_warn() {
    echo -e "${YELLOW}⚠${NC} $1"
    ((WARNINGS++))
}

echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}Pub/Sub Message Flow Diagnostic${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo ""

# Get GCP project ID
if [ -z "$GCP_PROJECT_ID" ]; then
    GCP_PROJECT_ID=$(gcloud config get-value project 2>/dev/null)
fi

if [ -z "$GCP_PROJECT_ID" ]; then
    echo -e "${RED}ERROR: GCP_PROJECT_ID not set and could not be determined${NC}"
    exit 1
fi

echo "Project ID: $GCP_PROJECT_ID"
echo "Region: europe-southwest1"
echo ""

# ============================================================================
# 1. Check Pub/Sub Topic Exists
# ============================================================================
echo -e "${BLUE}[1/10] Checking Pub/Sub Topic...${NC}"
if gcloud pubsub topics describe click-events --project=$GCP_PROJECT_ID &>/dev/null; then
    check_pass "Topic 'click-events' exists"
else
    check_fail "Topic 'click-events' does not exist"
    echo "  Fix: gcloud pubsub topics create click-events --project=$GCP_PROJECT_ID"
fi
echo ""

# ============================================================================
# 2. Check Subscription Exists
# ============================================================================
echo -e "${BLUE}[2/10] Checking Pub/Sub Subscription...${NC}"
if gcloud pubsub subscriptions describe click-consumer-sub --project=$GCP_PROJECT_ID &>/dev/null; then
    check_pass "Subscription 'click-consumer-sub' exists"
else
    check_fail "Subscription 'click-consumer-sub' does not exist"
    echo "  Fix: Create subscription with push configuration (see DEBUG_PUBSUB.md)"
fi
echo ""

# ============================================================================
# 3. Check Subscription Push Config
# ============================================================================
echo -e "${BLUE}[3/10] Checking Subscription Push Configuration...${NC}"
if gcloud pubsub subscriptions describe click-consumer-sub --project=$GCP_PROJECT_ID &>/dev/null; then
    PUSH_ENDPOINT=$(gcloud pubsub subscriptions describe click-consumer-sub \
        --project=$GCP_PROJECT_ID --format='value(pushConfig.pushEndpoint)' 2>/dev/null || echo "")

    if [ -z "$PUSH_ENDPOINT" ]; then
        check_fail "Subscription is not configured for PUSH delivery"
        echo "  Current mode: PULL (messages not being pushed to consumer)"
        echo "  Fix: Reconfigure subscription with --push-endpoint (see DEBUG_PUBSUB.md)"
    elif [[ "$PUSH_ENDPOINT" == *"/process" ]]; then
        check_pass "Push endpoint configured: $PUSH_ENDPOINT"
    else
        check_fail "Push endpoint doesn't end with /process: $PUSH_ENDPOINT"
    fi
else
    check_warn "Subscription doesn't exist, skipping push config check"
fi
echo ""

# ============================================================================
# 4. Check Backend Service Exists
# ============================================================================
echo -e "${BLUE}[4/10] Checking Backend Cloud Run Service...${NC}"
if gcloud run services describe clicker-backend --region=europe-southwest1 --project=$GCP_PROJECT_ID &>/dev/null; then
    BACKEND_URL=$(gcloud run services describe clicker-backend \
        --region=europe-southwest1 --project=$GCP_PROJECT_ID \
        --format='value(status.url)' 2>/dev/null)
    check_pass "Backend service deployed: $BACKEND_URL"
else
    check_fail "Backend service 'clicker-backend' not found in Cloud Run"
fi
echo ""

# ============================================================================
# 5. Check Consumer Service Exists
# ============================================================================
echo -e "${BLUE}[5/10] Checking Consumer Cloud Run Service...${NC}"
if gcloud run services describe clicker-consumer --region=europe-southwest1 --project=$GCP_PROJECT_ID &>/dev/null; then
    CONSUMER_URL=$(gcloud run services describe clicker-consumer \
        --region=europe-southwest1 --project=$GCP_PROJECT_ID \
        --format='value(status.url)' 2>/dev/null)
    check_pass "Consumer service deployed: $CONSUMER_URL"
else
    check_fail "Consumer service 'clicker-consumer' not found in Cloud Run"
fi
echo ""

# ============================================================================
# 6. Check Subscription Endpoint Matches Consumer URL
# ============================================================================
echo -e "${BLUE}[6/10] Checking Subscription Endpoint vs Consumer URL...${NC}"
if [ -n "$PUSH_ENDPOINT" ] && [ -n "$CONSUMER_URL" ]; then
    EXPECTED_ENDPOINT="$CONSUMER_URL/process"
    if [ "$PUSH_ENDPOINT" = "$EXPECTED_ENDPOINT" ]; then
        check_pass "Endpoint matches consumer URL"
    else
        check_fail "Endpoint mismatch!"
        echo "  Expected: $EXPECTED_ENDPOINT"
        echo "  Actual:   $PUSH_ENDPOINT"
        echo "  Fix: Update subscription with correct URL"
    fi
elif [ -z "$PUSH_ENDPOINT" ]; then
    check_warn "Push endpoint not configured, skipping match check"
elif [ -z "$CONSUMER_URL" ]; then
    check_warn "Consumer service not deployed, skipping match check"
fi
echo ""

# ============================================================================
# 7. Check Backend Service Account Permissions
# ============================================================================
echo -e "${BLUE}[7/10] Checking Backend Service Account Permissions...${NC}"
BACKEND_SA=$(gcloud iam service-accounts list --project=$GCP_PROJECT_ID \
    --filter="displayName:clicker-backend" --format="value(email)" 2>/dev/null || echo "")

if [ -z "$BACKEND_SA" ]; then
    check_warn "Backend service account not found"
else
    if gcloud projects get-iam-policy $GCP_PROJECT_ID \
        --flatten="bindings[].members" \
        --filter="bindings.role:roles/pubsub.publisher AND bindings.members:$BACKEND_SA" \
        --format="value(bindings.members)" &>/dev/null | grep -q "$BACKEND_SA"; then
        check_pass "Backend has pubsub.publisher role: $BACKEND_SA"
    else
        check_fail "Backend missing pubsub.publisher role"
        echo "  Service account: $BACKEND_SA"
        echo "  Fix: gcloud projects add-iam-policy-binding $GCP_PROJECT_ID --member=serviceAccount:$BACKEND_SA --role=roles/pubsub.publisher"
    fi
fi
echo ""

# ============================================================================
# 8. Check Consumer Service Account Permissions
# ============================================================================
echo -e "${BLUE}[8/10] Checking Consumer Service Account Permissions...${NC}"
CONSUMER_SA=$(gcloud iam service-accounts list --project=$GCP_PROJECT_ID \
    --filter="displayName:clicker-consumer" --format="value(email)" 2>/dev/null || echo "")

if [ -z "$CONSUMER_SA" ]; then
    check_warn "Consumer service account not found"
else
    # Check pubsub.subscriber
    if gcloud projects get-iam-policy $GCP_PROJECT_ID \
        --flatten="bindings[].members" \
        --filter="bindings.role:roles/pubsub.subscriber AND bindings.members:$CONSUMER_SA" \
        --format="value(bindings.members)" &>/dev/null | grep -q "$CONSUMER_SA"; then
        check_pass "Consumer has pubsub.subscriber role: $CONSUMER_SA"
    else
        check_warn "Consumer missing pubsub.subscriber role (may not be critical)"
    fi
fi
echo ""

# ============================================================================
# 9. Check Backend Can Access Pub/Sub Topic
# ============================================================================
echo -e "${BLUE}[9/10] Checking Backend Logs for Publisher Initialization...${NC}"
BACKEND_LOGS=$(gcloud run services logs read clicker-backend \
    --region=europe-southwest1 --project=$GCP_PROJECT_ID --limit=50 2>/dev/null || echo "")

if echo "$BACKEND_LOGS" | grep -q "Pub/Sub publisher initialized"; then
    check_pass "Backend successfully initialized Pub/Sub publisher"
elif echo "$BACKEND_LOGS" | grep -q "Failed to initialize Pub/Sub publisher"; then
    check_fail "Backend failed to initialize Pub/Sub publisher"
    echo "  Error details:"
    echo "$BACKEND_LOGS" | grep "Failed to initialize" | head -3
elif echo "$BACKEND_LOGS" | grep -q "does not exist"; then
    check_fail "Topic does not exist (backend logs show topic error)"
else
    check_warn "Cannot determine publisher status from logs (may be normal if no clicks sent yet)"
fi
echo ""

# ============================================================================
# 10. Check Consumer Health
# ============================================================================
echo -e "${BLUE}[10/10] Checking Consumer Service Health...${NC}"
if [ -n "$CONSUMER_URL" ]; then
    HEALTH_RESPONSE=$(curl -s -w "\n%{http_code}" "$CONSUMER_URL/health" 2>/dev/null || echo "connection_error")
    HTTP_CODE=$(echo "$HEALTH_RESPONSE" | tail -1)
    RESPONSE_BODY=$(echo "$HEALTH_RESPONSE" | head -n -1)

    if [ "$HTTP_CODE" = "200" ]; then
        check_pass "Consumer /health endpoint responding (HTTP 200)"
        echo "  Response: $RESPONSE_BODY"
    else
        check_fail "Consumer /health endpoint returned HTTP $HTTP_CODE"
    fi
else
    check_warn "Consumer service not found, skipping health check"
fi
echo ""

# ============================================================================
# Summary
# ============================================================================
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo -e "Results: ${GREEN}$PASSED passed${NC}, ${RED}$FAILED failed${NC}, ${YELLOW}$WARNINGS warnings${NC}"
echo -e "${BLUE}════════════════════════════════════════════════════════════${NC}"
echo ""

if [ $FAILED -eq 0 ] && [ $WARNINGS -eq 0 ]; then
    echo -e "${GREEN}✓ All checks passed! Message flow should be working.${NC}"
    echo ""
    echo "Next steps:"
    echo "1. Send a test click to the backend: curl $BACKEND_URL/click?country=US&ip=1.2.3.4"
    echo "2. Check the consumer logs for processing: gcloud run services logs read clicker-consumer --limit=20"
    echo "3. Verify the counter in Firestore increased"
    echo ""
    exit 0
elif [ $FAILED -eq 0 ]; then
    echo -e "${YELLOW}⚠ Some warnings found, but no failures. Message flow might be working.${NC}"
    exit 0
else
    echo -e "${RED}✗ Critical failures detected. Message flow is broken.${NC}"
    echo ""
    echo "Fix the failures above and re-run this script."
    echo "For detailed help, see: DEBUG_PUBSUB.md"
    echo ""
    exit 1
fi
