#!/bin/bash
#
# OpenIncident E2E Test Script
#
# Tests the complete alert-to-incident workflow:
# 1. Health/readiness checks
# 2. Prometheus webhook ingestion
# 3. Incident auto-creation
# 4. Alert storage and linking
# 5. Incident API operations (list, get, update)
# 6. Status transitions
#
# Usage:
#   ./scripts/e2e-test.sh                    # Run full test suite
#   ./scripts/e2e-test.sh --verbose          # Run with detailed output
#   ./scripts/e2e-test.sh --cleanup-only     # Only cleanup test data
#

# Configuration
OPENINCIDENT_URL="${OPENINCIDENT_URL:-http://localhost:8080}"
VERBOSE="${VERBOSE:-0}"
CLEANUP_ONLY="${CLEANUP_ONLY:-0}"

# Parse arguments
for arg in "$@"; do
    case "$arg" in
        --verbose|-v)
            VERBOSE=1
            ;;
        --cleanup-only)
            CLEANUP_ONLY=1
            ;;
        --help|-h)
            echo "Usage: $0 [options]"
            echo ""
            echo "Options:"
            echo "  --verbose, -v       Show detailed output"
            echo "  --cleanup-only      Only cleanup test data and exit"
            echo "  --help, -h          Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  OPENINCIDENT_URL    OpenIncident URL (default: http://localhost:8080)"
            exit 0
            ;;
        *)
            echo "Unknown option: $arg"
            echo "Run '$0 --help' for usage information"
            exit 1
            ;;
    esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# Test results tracking
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_TOTAL=0

# Test data to cleanup
INCIDENT_IDS=()
ALERT_IDS=()

# Helper functions
print_header() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

print_section() {
    echo ""
    echo -e "${CYAN}▶ $1${NC}"
}

print_success() {
    echo -e "  ${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "  ${RED}✗${NC} $1"
}

print_info() {
    echo -e "  ${YELLOW}ℹ${NC} $1"
}

print_verbose() {
    if [ "$VERBOSE" = "1" ]; then
        echo -e "  ${NC}  $1${NC}"
    fi
}

# Test assertion functions
assert_equals() {
    local expected="$1"
    local actual="$2"
    local test_name="$3"

    TESTS_TOTAL=$((TESTS_TOTAL + 1))

    if [ "$expected" = "$actual" ]; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        print_success "$test_name"
        print_verbose "Expected: $expected | Actual: $actual"
        return 0
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        print_error "$test_name"
        print_error "Expected: $expected"
        print_error "Actual: $actual"
        return 1
    fi
}

assert_not_empty() {
    local value="$1"
    local test_name="$2"

    TESTS_TOTAL=$((TESTS_TOTAL + 1))

    if [ -n "$value" ] && [ "$value" != "null" ]; then
        TESTS_PASSED=$((TESTS_PASSED + 1))
        print_success "$test_name"
        print_verbose "Value: $value"
        return 0
    else
        TESTS_FAILED=$((TESTS_FAILED + 1))
        print_error "$test_name"
        print_error "Value is empty or null"
        return 1
    fi
}

assert_http_status() {
    local expected="$1"
    local actual="$2"
    local test_name="$3"

    assert_equals "$expected" "$actual" "$test_name (HTTP $expected)"
}

# Cleanup function
cleanup_test_data() {
    print_section "Cleaning Up Test Data"

    # Note: In production, we'd delete specific test incidents/alerts
    # For now, just show what would be cleaned
    if [ ${#INCIDENT_IDS[@]} -gt 0 ]; then
        print_info "Created ${#INCIDENT_IDS[@]} incident(s) during test"
        for id in "${INCIDENT_IDS[@]}"; do
            print_verbose "Incident ID: $id"
        done
    fi

    if [ ${#ALERT_IDS[@]} -gt 0 ]; then
        print_info "Created ${#ALERT_IDS[@]} alert(s) during test"
    fi

    print_info "To fully reset database, run: docker-compose down -v && docker-compose up -d"
}

# Test 1: Health Check
test_health_endpoint() {
    print_section "Test 1: Health Endpoint"

    RESPONSE=$(curl -s -w "\n%{http_code}" "$OPENINCIDENT_URL/health")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "GET $OPENINCIDENT_URL/health"
    print_verbose "Response: $BODY"

    assert_http_status "200" "$HTTP_CODE" "Health endpoint responds"

    STATUS=$(echo "$BODY" | jq -r '.status' 2>/dev/null || echo "")
    assert_equals "ok" "$STATUS" "Health status is 'ok'"
}

# Test 2: Readiness Check
test_ready_endpoint() {
    print_section "Test 2: Readiness Endpoint"

    RESPONSE=$(curl -s -w "\n%{http_code}" "$OPENINCIDENT_URL/ready")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "GET $OPENINCIDENT_URL/ready"
    print_verbose "Response: $BODY"

    assert_http_status "200" "$HTTP_CODE" "Ready endpoint responds"

    STATUS=$(echo "$BODY" | jq -r '.status' 2>/dev/null || echo "")
    assert_equals "ready" "$STATUS" "Status is 'ready'"

    DB_STATUS=$(echo "$BODY" | jq -r '.database' 2>/dev/null || echo "")
    assert_equals "ok" "$DB_STATUS" "Database is ready"

    REDIS_STATUS=$(echo "$BODY" | jq -r '.redis' 2>/dev/null || echo "")
    assert_equals "ok" "$REDIS_STATUS" "Redis is ready"
}

# Test 3: Prometheus Webhook Ingestion
test_prometheus_webhook() {
    print_section "Test 3: Prometheus Webhook Ingestion"

    # Generate unique IDs for this test run
    UNIQUE_ID="$(date +%s)-$$-$RANDOM"
    ALERT_NAME="E2ETest$(date +%s)"

    # Create test payload
    PAYLOAD=$(cat <<EOF
{
  "version": "4",
  "status": "firing",
  "alerts": [
    {
      "status": "firing",
      "labels": {
        "alertname": "${ALERT_NAME}",
        "severity": "critical",
        "instance": "test-instance",
        "job": "e2e-test"
      },
      "annotations": {
        "summary": "E2E test alert for validation",
        "description": "This alert is generated by the E2E test suite"
      },
      "startsAt": "2026-02-07T10:00:00Z",
      "endsAt": "0001-01-01T00:00:00Z",
      "fingerprint": "e2e-test-${UNIQUE_ID}"
    }
  ]
}
EOF
)

    # Save for validation
    TEST_ALERT_NAME="$ALERT_NAME"

    print_verbose "POST $OPENINCIDENT_URL/api/v1/webhooks/prometheus"
    print_verbose "Payload: $PAYLOAD"

    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
        "$OPENINCIDENT_URL/api/v1/webhooks/prometheus" \
        -H "Content-Type: application/json" \
        -d "$PAYLOAD")

    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "Response: $BODY"

    assert_http_status "200" "$HTTP_CODE" "Webhook accepted"

    RECEIVED=$(echo "$BODY" | jq -r '.received' 2>/dev/null || echo "0")
    assert_equals "1" "$RECEIVED" "1 alert received"

    INCIDENTS_CREATED=$(echo "$BODY" | jq -r '.incidents_created' 2>/dev/null || echo "0")
    assert_equals "1" "$INCIDENTS_CREATED" "1 incident auto-created"

    # Save response for later tests
    WEBHOOK_RESPONSE="$BODY"
}

# Test 4: List Incidents
test_list_incidents() {
    print_section "Test 4: List Incidents API"

    RESPONSE=$(curl -s -w "\n%{http_code}" "$OPENINCIDENT_URL/api/v1/incidents")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "GET $OPENINCIDENT_URL/api/v1/incidents"
    print_verbose "Response: $BODY"

    assert_http_status "200" "$HTTP_CODE" "List incidents endpoint responds"

    # Parse paginated response
    TOTAL=$(echo "$BODY" | jq -r '.total' 2>/dev/null || echo "0")
    assert_not_empty "$TOTAL" "Total count is present"

    DATA=$(echo "$BODY" | jq -r '.data' 2>/dev/null || echo "[]")
    COUNT=$(echo "$DATA" | jq 'length' 2>/dev/null || echo "0")

    if [ "$COUNT" -gt 0 ]; then
        print_success "Found $COUNT incident(s)"

        # Get the first incident for further tests
        INCIDENT_ID=$(echo "$DATA" | jq -r '.[0].id' 2>/dev/null || echo "")
        INCIDENT_NUMBER=$(echo "$DATA" | jq -r '.[0].incident_number' 2>/dev/null || echo "")
        INCIDENT_STATUS=$(echo "$DATA" | jq -r '.[0].status' 2>/dev/null || echo "")

        assert_not_empty "$INCIDENT_ID" "Incident has ID"
        assert_not_empty "$INCIDENT_NUMBER" "Incident has number"
        assert_equals "triggered" "$INCIDENT_STATUS" "Incident status is 'triggered'"

        # Save for cleanup
        INCIDENT_IDS+=("$INCIDENT_ID")

        # Save for later tests
        FIRST_INCIDENT_ID="$INCIDENT_ID"
        FIRST_INCIDENT_NUMBER="$INCIDENT_NUMBER"

        print_verbose "Incident ID: $INCIDENT_ID"
        print_verbose "Incident Number: $INCIDENT_NUMBER"
    else
        print_error "No incidents found (expected at least 1 from webhook)"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Test 5: Get Incident by ID
test_get_incident_by_id() {
    print_section "Test 5: Get Incident by ID"

    if [ -z "$FIRST_INCIDENT_ID" ]; then
        print_error "Skipping: No incident ID from previous test"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi

    RESPONSE=$(curl -s -w "\n%{http_code}" "$OPENINCIDENT_URL/api/v1/incidents/$FIRST_INCIDENT_ID")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "GET $OPENINCIDENT_URL/api/v1/incidents/$FIRST_INCIDENT_ID"
    print_verbose "Response: $BODY"

    assert_http_status "200" "$HTTP_CODE" "Get incident by UUID"

    ID=$(echo "$BODY" | jq -r '.id' 2>/dev/null || echo "")
    assert_equals "$FIRST_INCIDENT_ID" "$ID" "Incident ID matches"

    TITLE=$(echo "$BODY" | jq -r '.title' 2>/dev/null || echo "")
    assert_equals "$TEST_ALERT_NAME" "$TITLE" "Incident title from alert"

    SEVERITY=$(echo "$BODY" | jq -r '.severity' 2>/dev/null || echo "")
    assert_equals "critical" "$SEVERITY" "Incident severity is 'critical'"

    CREATED_BY_TYPE=$(echo "$BODY" | jq -r '.created_by_type' 2>/dev/null || echo "")
    assert_equals "system" "$CREATED_BY_TYPE" "Created by system"

    CREATED_BY_ID=$(echo "$BODY" | jq -r '.created_by_id' 2>/dev/null || echo "")
    assert_equals "alertmanager" "$CREATED_BY_ID" "Created by alertmanager"

    # Check detail response includes alerts and timeline
    ALERTS=$(echo "$BODY" | jq -r '.alerts' 2>/dev/null || echo "null")
    if [ "$ALERTS" != "null" ]; then
        print_success "Response includes alerts array"
    else
        print_info "Response does not include alerts (may need separate endpoint)"
    fi

    TIMELINE=$(echo "$BODY" | jq -r '.timeline' 2>/dev/null || echo "null")
    if [ "$TIMELINE" != "null" ]; then
        print_success "Response includes timeline array"
    else
        print_info "Response does not include timeline (may need separate endpoint)"
    fi
}

# Test 6: Get Incident by Number
test_get_incident_by_number() {
    print_section "Test 6: Get Incident by Number"

    if [ -z "$FIRST_INCIDENT_NUMBER" ]; then
        print_error "Skipping: No incident number from previous test"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi

    RESPONSE=$(curl -s -w "\n%{http_code}" "$OPENINCIDENT_URL/api/v1/incidents/$FIRST_INCIDENT_NUMBER")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "GET $OPENINCIDENT_URL/api/v1/incidents/$FIRST_INCIDENT_NUMBER"
    print_verbose "Response: $BODY"

    assert_http_status "200" "$HTTP_CODE" "Get incident by number"

    NUMBER=$(echo "$BODY" | jq -r '.incident_number' 2>/dev/null || echo "")
    assert_equals "$FIRST_INCIDENT_NUMBER" "$NUMBER" "Incident number matches"

    ID=$(echo "$BODY" | jq -r '.id' 2>/dev/null || echo "")
    assert_equals "$FIRST_INCIDENT_ID" "$ID" "Same incident as by UUID"
}

# Test 7: Update Incident Status
# NOTE: This test currently fails due to a known transaction handling bug in the service layer.
# The service creates a transaction but repositories don't use the tx context, so updates are
# logged but not persisted. See backend/internal/api/handlers/incidents_test.go for details.
test_update_incident_status() {
    print_section "Test 7: Update Incident Status"

    if [ -z "$FIRST_INCIDENT_ID" ]; then
        print_error "Skipping: No incident ID from previous test"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi

    # Test transition: triggered -> acknowledged
    UPDATE_PAYLOAD='{"status": "acknowledged"}'

    print_verbose "PATCH $OPENINCIDENT_URL/api/v1/incidents/$FIRST_INCIDENT_ID"
    print_verbose "Payload: $UPDATE_PAYLOAD"

    RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH \
        "$OPENINCIDENT_URL/api/v1/incidents/$FIRST_INCIDENT_ID" \
        -H "Content-Type: application/json" \
        -d "$UPDATE_PAYLOAD")

    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "Response: $BODY"

    assert_http_status "200" "$HTTP_CODE" "Update status to 'acknowledged'"

    STATUS=$(echo "$BODY" | jq -r '.status' 2>/dev/null || echo "")
    assert_equals "acknowledged" "$STATUS" "Status updated to 'acknowledged'"

    ACKNOWLEDGED_AT=$(echo "$BODY" | jq -r '.acknowledged_at' 2>/dev/null || echo "null")
    assert_not_empty "$ACKNOWLEDGED_AT" "acknowledged_at timestamp set"
}

# Test 8: Invalid Status Transition
# NOTE: This test also fails due to the transaction bug - since Test 7 didn't actually persist
# the status change to 'acknowledged', the incident is still 'triggered', so this "backward"
# transition actually succeeds (triggered -> triggered), when it should fail.
test_invalid_status_transition() {
    print_section "Test 8: Invalid Status Transition"

    if [ -z "$FIRST_INCIDENT_ID" ]; then
        print_error "Skipping: No incident ID from previous test"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi

    # Try invalid transition: acknowledged -> triggered (backward)
    UPDATE_PAYLOAD='{"status": "triggered"}'

    print_verbose "PATCH $OPENINCIDENT_URL/api/v1/incidents/$FIRST_INCIDENT_ID"
    print_verbose "Payload: $UPDATE_PAYLOAD"

    RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH \
        "$OPENINCIDENT_URL/api/v1/incidents/$FIRST_INCIDENT_ID" \
        -H "Content-Type: application/json" \
        -d "$UPDATE_PAYLOAD")

    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "Response: $BODY"

    assert_http_status "409" "$HTTP_CODE" "Invalid transition rejected"

    ERROR=$(echo "$BODY" | jq -r '.error' 2>/dev/null || echo "")
    assert_not_empty "$ERROR" "Error message returned"
}

# Test 9: Update Incident Severity
test_update_incident_severity() {
    print_section "Test 9: Update Incident Severity"

    if [ -z "$FIRST_INCIDENT_ID" ]; then
        print_error "Skipping: No incident ID from previous test"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi

    UPDATE_PAYLOAD='{"severity": "high"}'

    print_verbose "PATCH $OPENINCIDENT_URL/api/v1/incidents/$FIRST_INCIDENT_ID"
    print_verbose "Payload: $UPDATE_PAYLOAD"

    RESPONSE=$(curl -s -w "\n%{http_code}" -X PATCH \
        "$OPENINCIDENT_URL/api/v1/incidents/$FIRST_INCIDENT_ID" \
        -H "Content-Type: application/json" \
        -d "$UPDATE_PAYLOAD")

    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "Response: $BODY"

    assert_http_status "200" "$HTTP_CODE" "Update severity"

    SEVERITY=$(echo "$BODY" | jq -r '.severity' 2>/dev/null || echo "")
    assert_equals "high" "$SEVERITY" "Severity updated to 'high'"
}

# Test 10: Pagination
test_pagination() {
    print_section "Test 10: Pagination"

    RESPONSE=$(curl -s -w "\n%{http_code}" "$OPENINCIDENT_URL/api/v1/incidents?page=1&limit=5")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "GET $OPENINCIDENT_URL/api/v1/incidents?page=1&limit=5"
    print_verbose "Response: $BODY"

    assert_http_status "200" "$HTTP_CODE" "Pagination query succeeds"

    LIMIT=$(echo "$BODY" | jq -r '.limit' 2>/dev/null || echo "")
    assert_equals "5" "$LIMIT" "Limit parameter respected"

    DATA=$(echo "$BODY" | jq -r '.data | length' 2>/dev/null || echo "0")

    if [ "$DATA" -le 5 ]; then
        print_success "Returned $DATA incident(s) (≤ limit)"
    else
        print_error "Returned $DATA incident(s) (> limit of 5)"
        TESTS_FAILED=$((TESTS_FAILED + 1))
    fi
}

# Test 11: Filter by Status
test_filter_by_status() {
    print_section "Test 11: Filter by Status"

    RESPONSE=$(curl -s -w "\n%{http_code}" "$OPENINCIDENT_URL/api/v1/incidents?status=acknowledged")
    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')

    print_verbose "GET $OPENINCIDENT_URL/api/v1/incidents?status=acknowledged"
    print_verbose "Response: $BODY"

    assert_http_status "200" "$HTTP_CODE" "Status filter query succeeds"

    DATA=$(echo "$BODY" | jq -r '.data' 2>/dev/null || echo "[]")
    COUNT=$(echo "$DATA" | jq 'length' 2>/dev/null || echo "0")

    if [ "$COUNT" -gt 0 ]; then
        print_success "Found $COUNT acknowledged incident(s)"

        # Verify all returned incidents have status=acknowledged
        ALL_ACKNOWLEDGED=$(echo "$DATA" | jq 'all(.status == "acknowledged")' 2>/dev/null || echo "false")
        if [ "$ALL_ACKNOWLEDGED" = "true" ]; then
            print_success "All returned incidents have status 'acknowledged'"
        else
            print_error "Some returned incidents do not have status 'acknowledged'"
            TESTS_FAILED=$((TESTS_FAILED + 1))
        fi
    else
        print_info "No acknowledged incidents found (expected at least 1)"
    fi
}

# Print summary
print_summary() {
    print_header "Test Summary"

    echo ""
    echo -e "  Total Tests:  ${TESTS_TOTAL}"
    echo -e "  ${GREEN}Passed:       ${TESTS_PASSED}${NC}"

    if [ "$TESTS_FAILED" -gt 0 ]; then
        echo -e "  ${RED}Failed:       ${TESTS_FAILED}${NC}"
    else
        echo -e "  ${GREEN}Failed:       ${TESTS_FAILED}${NC}"
    fi

    echo ""

    if [ "$TESTS_FAILED" -eq 0 ]; then
        echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${GREEN}✓ ALL TESTS PASSED${NC}"
        echo -e "${GREEN}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        return 0
    else
        echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        echo -e "${RED}✗ SOME TESTS FAILED${NC}"
        echo -e "${RED}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
        return 1
    fi
}

# Main execution
main() {
    print_header "OpenIncident E2E Test Suite"
    echo ""
    echo -e "  Target: ${CYAN}$OPENINCIDENT_URL${NC}"
    echo -e "  Mode:   ${CYAN}$([ "$VERBOSE" = "1" ] && echo "Verbose" || echo "Standard")${NC}"

    if [ "$CLEANUP_ONLY" = "1" ]; then
        cleanup_test_data
        exit 0
    fi

    # Run all tests
    test_health_endpoint
    test_ready_endpoint
    test_prometheus_webhook
    test_list_incidents
    test_get_incident_by_id
    test_get_incident_by_number
    test_update_incident_status
    test_invalid_status_transition
    test_update_incident_severity
    test_pagination
    test_filter_by_status

    # Print summary
    print_summary
    EXIT_CODE=$?

    # Cleanup
    cleanup_test_data

    exit $EXIT_CODE
}

# Run main function
main
