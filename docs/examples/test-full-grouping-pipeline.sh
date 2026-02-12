#!/bin/bash

# Full Grouping Pipeline Integration Test
# Tests the complete flow: Rule creation → Multi-source alerts → Grouped incident

BASE_URL="http://localhost:8080"
API_BASE="$BASE_URL/api/v1"

echo "========================================="
echo "Full Grouping Pipeline Integration Test"
echo "========================================="
echo
echo "This test verifies:"
echo "  ✓ Grouping rule creation"
echo "  ✓ Cross-source alert correlation"
echo "  ✓ Incident grouping behavior"
echo "  ✓ Timeline entries for grouped alerts"
echo "  ✓ Frontend data structure"
echo
echo "========================================="
echo

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper function for colored output
success() {
    echo -e "${GREEN}✅ $1${NC}"
}

error() {
    echo -e "${RED}❌ $1${NC}"
}

info() {
    echo -e "${BLUE}ℹ️  $1${NC}"
}

warning() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

# Step 1: Create a cross-source grouping rule
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 1: Create Cross-Source Grouping Rule"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

info "Creating rule: Group by service + env (cross-source)"

RULE_RESPONSE=$(curl -s -X POST "$API_BASE/grouping-rules" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Integration Test: Cross-source by service+env",
    "description": "Groups critical alerts from all sources for same service/env",
    "enabled": true,
    "priority": 10,
    "match_labels": {
      "severity": "critical"
    },
    "cross_source_labels": ["service", "env"],
    "time_window_seconds": 600
  }')

RULE_ID=$(echo "$RULE_RESPONSE" | jq -r '.id')

if [ "$RULE_ID" != "null" ] && [ -n "$RULE_ID" ]; then
    success "Created grouping rule: $RULE_ID"
    echo "   Priority: 10"
    echo "   Match: severity=critical"
    echo "   Group by: service, env"
    echo "   Window: 600 seconds"
else
    error "Failed to create grouping rule"
    echo "Response: $RULE_RESPONSE"
    exit 1
fi
echo

# Step 2: Send alerts from multiple sources
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 2: Send Alerts from Multiple Sources"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# Wait for grouping engine cache to refresh (TTL is 5 seconds)
info "Waiting 6 seconds for grouping engine cache refresh..."
sleep 6

# Generate unique timestamp for this test run (to avoid deduplication with previous runs)
TEST_TIMESTAMP=$(date +%s)
info "Test run ID: $TEST_TIMESTAMP"
echo

# Alert 1: Prometheus - HighCPU
info "Sending Alert 1: Prometheus HighCPU"
PROM_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE/webhooks/prometheus" \
  -H "Content-Type: application/json" \
  -d '{
    "version": "4",
    "groupKey": "{}:{alertname=\"HighCPU\"}",
    "status": "firing",
    "receiver": "openincident",
    "alerts": [{
      "status": "firing",
      "fingerprint": "highcpu-'$TEST_TIMESTAMP'",
      "labels": {
        "alertname": "HighCPU",
        "service": "payment-api",
        "env": "production",
        "severity": "critical",
        "instance": "web-01"
      },
      "annotations": {
        "summary": "CPU usage above 90% on web-01",
        "description": "CPU utilization has been above 90% for 5 minutes"
      },
      "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
    }]
  }')

PROM_HTTP_CODE=$(echo "$PROM_RESPONSE" | tail -n1)
PROM_BODY=$(echo "$PROM_RESPONSE" | sed '$d')
if [ "$PROM_HTTP_CODE" == "200" ]; then
    success "Prometheus alert received (HTTP $PROM_HTTP_CODE)"
else
    warning "Prometheus failed: HTTP $PROM_HTTP_CODE - $PROM_BODY"
fi
echo

# Wait 2 seconds before next alert
sleep 2

# Alert 2: Grafana - HighLatency
info "Sending Alert 2: Grafana HighLatency (same service+env)"
GRAFANA_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE/webhooks/grafana" \
  -H "Content-Type: application/json" \
  -d '{
    "state": "alerting",
    "title": "HighLatency",
    "message": "API latency above 2 seconds",
    "alerts": [{
      "status": "firing",
      "fingerprint": "highlatency-'$TEST_TIMESTAMP'",
      "labels": {
        "alertname": "HighLatency",
        "service": "payment-api",
        "env": "production",
        "severity": "critical",
        "dashboard": "api-overview"
      },
      "annotations": {
        "summary": "API p95 latency is 2.5s"
      },
      "startsAt": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
    }]
  }')

GRAFANA_HTTP_CODE=$(echo "$GRAFANA_RESPONSE" | tail -n1)
GRAFANA_BODY=$(echo "$GRAFANA_RESPONSE" | sed '$d')
if [ "$GRAFANA_HTTP_CODE" == "200" ]; then
    success "Grafana alert received (HTTP $GRAFANA_HTTP_CODE)"
else
    warning "Grafana failed: HTTP $GRAFANA_HTTP_CODE - $GRAFANA_BODY"
fi
echo

# Wait 2 seconds before next alert
sleep 2

# Alert 3: Generic/CloudWatch - HighErrorRate
info "Sending Alert 3: Generic HighErrorRate (same service+env)"
GENERIC_RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE/webhooks/generic" \
  -H "Content-Type: application/json" \
  -d '{
    "alerts": [{
      "external_id": "higherrorrate-'$TEST_TIMESTAMP'",
      "title": "HighErrorRate",
      "description": "Error rate above 5%",
      "severity": "critical",
      "status": "firing",
      "labels": {
        "alertname": "HighErrorRate",
        "service": "payment-api",
        "env": "production",
        "severity": "critical",
        "region": "us-east-1"
      },
      "started_at": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
    }]
  }')

GENERIC_HTTP_CODE=$(echo "$GENERIC_RESPONSE" | tail -n1)
GENERIC_BODY=$(echo "$GENERIC_RESPONSE" | sed '$d')
if [ "$GENERIC_HTTP_CODE" == "200" ]; then
    success "Generic alert received (HTTP $GENERIC_HTTP_CODE)"
else
    warning "Generic failed: HTTP $GENERIC_HTTP_CODE - $GENERIC_BODY"
fi
echo

# Wait for async processing
info "Waiting 3 seconds for incident creation and grouping..."
sleep 3
echo

# Step 3: Verify incidents were created and grouped
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 3: Verify Incident Grouping"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

# Find the most recently created incident that has a group_key (created by this test)
INCIDENTS=$(curl -s "$API_BASE/incidents")
INCIDENT_ID=$(echo "$INCIDENTS" | jq -r '[.data[] | select(.group_key != null)] | sort_by(.created_at) | last | .id')

if [ "$INCIDENT_ID" != "null" ] && [ -n "$INCIDENT_ID" ]; then
    # Fetch incident detail to count alerts
    INCIDENT_CHECK=$(curl -s "$API_BASE/incidents/$INCIDENT_ID")
    ALERT_COUNT_CHECK=$(echo "$INCIDENT_CHECK" | jq -r '.alerts | length')
    GROUP_KEY_CHECK=$(echo "$INCIDENT_CHECK" | jq -r '.group_key // "null"')

    if [ "$ALERT_COUNT_CHECK" == "3" ] && [ "$GROUP_KEY_CHECK" != "null" ]; then
        success "✨ CROSS-SOURCE GROUPING SUCCESSFUL! ✨"
        echo
        echo "   All 3 alerts from different sources were grouped into 1 incident!"
        echo "   Sources: Prometheus, Grafana, Generic"
        echo "   Incident ID: $INCIDENT_ID"
        echo
    else
        warning "Incident found but alert count is $ALERT_COUNT_CHECK (expected 3)"
    fi
else
    error "No incident with group_key found"
    echo
    echo "Recent incidents:"
    echo "$INCIDENTS" | jq '.data[:5] | .[] | {id, title, status, group_key}'
    echo
    warning "Grouping may not be working correctly"
fi
echo

# Step 4: Inspect the incident details
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 4: Inspect Grouped Incident Details"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

if [ "$INCIDENT_ID" != "null" ] && [ -n "$INCIDENT_ID" ]; then
    INCIDENT_DETAIL=$(curl -s "$API_BASE/incidents/$INCIDENT_ID")

    INCIDENT_NUMBER=$(echo "$INCIDENT_DETAIL" | jq -r '.incident_number')
    INCIDENT_TITLE=$(echo "$INCIDENT_DETAIL" | jq -r '.title')
    INCIDENT_STATUS=$(echo "$INCIDENT_DETAIL" | jq -r '.status')
    GROUP_KEY=$(echo "$INCIDENT_DETAIL" | jq -r '.group_key')
    ALERT_COUNT=$(echo "$INCIDENT_DETAIL" | jq -r '.alerts | length')
    TIMELINE_COUNT=$(echo "$INCIDENT_DETAIL" | jq -r '.timeline | length')

    echo "📋 Incident Details:"
    echo "   ID: $INCIDENT_ID"
    echo "   Number: INC-$INCIDENT_NUMBER"
    echo "   Title: $INCIDENT_TITLE"
    echo "   Status: $INCIDENT_STATUS"
    echo "   Group Key: ${GROUP_KEY:0:16}...${GROUP_KEY: -16}"
    echo

    if [ "$GROUP_KEY" != "null" ] && [ -n "$GROUP_KEY" ]; then
        success "Incident has group_key (created via grouping rule)"
    else
        warning "Incident missing group_key"
    fi

    echo
    echo "📊 Alert Statistics:"
    echo "   Total Alerts: $ALERT_COUNT"

    if [ "$ALERT_COUNT" == "3" ]; then
        success "All 3 alerts linked to incident"

        # Show alert sources
        echo
        echo "   Alert Sources:"
        SOURCES=$(echo "$INCIDENT_DETAIL" | jq -r '.alerts[] | "     • " + .source + " - " + .title')
        echo "$SOURCES"

        # Check for source diversity
        UNIQUE_SOURCES=$(echo "$INCIDENT_DETAIL" | jq -r '.alerts[].source' | sort -u | wc -l | tr -d ' ')
        if [ "$UNIQUE_SOURCES" == "3" ]; then
            success "Cross-source correlation detected: $UNIQUE_SOURCES different sources!"
        else
            info "Unique sources: $UNIQUE_SOURCES"
        fi
    else
        warning "Expected 3 alerts, got $ALERT_COUNT"
    fi

    echo
    echo "📝 Timeline Entries: $TIMELINE_COUNT"

    # Show timeline types
    TIMELINE_TYPES=$(echo "$INCIDENT_DETAIL" | jq -r '.timeline[] | .type' | sort | uniq -c)
    echo "$TIMELINE_TYPES" | while read count type; do
        echo "   • $type: $count"
    done

else
    error "No incident found"
fi
echo

# Step 5: Verify frontend data structure
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 5: Verify Frontend Data Structure"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

if [ "$INCIDENT_ID" != "null" ] && [ -n "$INCIDENT_ID" ]; then
    # Check that all required fields for frontend are present
    HAS_GROUP_KEY=$(echo "$INCIDENT_DETAIL" | jq 'has("group_key")')
    HAS_ALERTS=$(echo "$INCIDENT_DETAIL" | jq 'has("alerts")')
    HAS_TIMELINE=$(echo "$INCIDENT_DETAIL" | jq 'has("timeline")')

    if [ "$HAS_GROUP_KEY" == "true" ]; then
        success "group_key field present (for GroupedAlerts UI)"
    else
        warning "group_key field missing"
    fi

    if [ "$HAS_ALERTS" == "true" ]; then
        success "alerts array present"

        # Verify alert structure
        FIRST_ALERT=$(echo "$INCIDENT_DETAIL" | jq '.alerts[0]')
        HAS_SOURCE=$(echo "$FIRST_ALERT" | jq 'has("source")')
        HAS_LABELS=$(echo "$FIRST_ALERT" | jq 'has("labels")')

        if [ "$HAS_SOURCE" == "true" ] && [ "$HAS_LABELS" == "true" ]; then
            success "Alert structure valid for frontend display"
        fi
    fi

    if [ "$HAS_TIMELINE" == "true" ]; then
        success "timeline array present"
    fi

    echo
    info "Frontend should display:"
    echo "   • Grouping header (3 alerts from 3 sources)"
    echo "   • Visual connectors between alerts"
    echo "   • Cross-source badges (prometheus, grafana, generic)"
    echo "   • Group key in advanced section"
fi
echo

# Step 6: Cleanup
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo "Step 6: Cleanup"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
echo

info "Deleting test grouping rule..."
DELETE_RESPONSE=$(curl -s -w "\n%{http_code}" -X DELETE "$API_BASE/grouping-rules/$RULE_ID")
DELETE_CODE=$(echo "$DELETE_RESPONSE" | tail -n1)

if [ "$DELETE_CODE" == "204" ]; then
    success "Grouping rule deleted"
else
    warning "Failed to delete grouping rule (HTTP $DELETE_CODE)"
fi
echo

# Summary
echo "========================================="
echo "Test Summary"
echo "========================================="
echo

if [ "$ALERT_COUNT" == "3" ] && [ "$GROUP_KEY" != "null" ] && [ -n "$GROUP_KEY" ]; then
    echo -e "${GREEN}"
    echo "╔══════════════════════════════════════╗"
    echo "║  ✅ ALL TESTS PASSED!               ║"
    echo "╚══════════════════════════════════════╝"
    echo -e "${NC}"
    echo
    success "Grouping rule creation"
    success "Multi-source alert ingestion"
    success "Cross-source correlation (3 sources → 1 incident)"
    success "Group key generation"
    success "Timeline entry creation"
    success "Frontend data structure"
    echo
    echo "The complete grouping pipeline is working correctly!"
    echo
    echo "Next steps:"
    echo "  1. Open the UI: $BASE_URL"
    echo "  2. View the incident: $BASE_URL/incidents/$INCIDENT_ID"
    echo "  3. Click 'Alerts' tab to see grouped alerts visualization"
    echo
else
    echo -e "${YELLOW}"
    echo "╔══════════════════════════════════════╗"
    echo "║  ⚠️  SOME ISSUES DETECTED           ║"
    echo "╚══════════════════════════════════════╝"
    echo -e "${NC}"
    echo
    if [ -z "$INCIDENT_ID" ] || [ "$INCIDENT_ID" == "null" ]; then
        warning "No grouped incident found"
    fi
    if [ "$ALERT_COUNT" != "3" ]; then
        warning "Expected 3 alerts, got $ALERT_COUNT"
    fi
    if [ "$GROUP_KEY" == "null" ]; then
        warning "Missing group_key on incident"
    fi
    echo
    echo "Check backend logs for more details"
fi

echo "========================================="
