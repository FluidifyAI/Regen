#!/bin/bash
#
# OpenIncident Test Alert Script
#
# Usage:
#   ./test-alerts.sh               # Send all test alerts
#   ./test-alerts.sh firing        # Send only firing alert
#   ./test-alerts.sh resolved      # Send only resolved alert
#   ./test-alerts.sh warning       # Send only warning alert
#   ./test-alerts.sh info          # Send only info alert
#   ./test-alerts.sh multiple      # Send multiple alerts
#   ./test-alerts.sh all           # Send all alerts
#

set -e

# Configuration
OPENINCIDENT_URL="${OPENINCIDENT_URL:-http://localhost:8080}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Helper functions
print_header() {
    echo -e "${BLUE}===================================================${NC}"
    echo -e "${BLUE}$1${NC}"
    echo -e "${BLUE}===================================================${NC}"
}

print_success() {
    echo -e "${GREEN}✓${NC} $1"
}

print_error() {
    echo -e "${RED}✗${NC} $1"
}

print_info() {
    echo -e "${YELLOW}ℹ${NC} $1"
}

# Check if OpenIncident is running
check_health() {
    print_header "Checking OpenIncident Health"

    if curl -s -f "$OPENINCIDENT_URL/health" > /dev/null; then
        print_success "OpenIncident is running at $OPENINCIDENT_URL"
    else
        print_error "OpenIncident is not responding at $OPENINCIDENT_URL"
        print_info "Make sure OpenIncident is running:"
        echo "  docker-compose up -d"
        echo "  OR"
        echo "  cd backend && go run ./cmd/openincident"
        exit 1
    fi

    # Check readiness
    READY=$(curl -s "$OPENINCIDENT_URL/ready")
    if echo "$READY" | grep -q '"status":"ready"'; then
        print_success "Database and Redis are ready"
    else
        print_error "OpenIncident is not ready"
        echo "$READY" | jq '.' 2>/dev/null || echo "$READY"
        exit 1
    fi

    echo ""
}

# Send alert from JSON file
send_alert() {
    local file="$1"
    local name="$2"

    if [ ! -f "$file" ]; then
        print_error "File not found: $file"
        return 1
    fi

    print_info "Sending $name..."

    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST \
        "$OPENINCIDENT_URL/api/v1/webhooks/prometheus" \
        -H "Content-Type: application/json" \
        -d @"$file")

    HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
    BODY=$(echo "$RESPONSE" | sed '$d')  # Remove last line (cross-platform)

    if [ "$HTTP_CODE" = "200" ] || [ "$HTTP_CODE" = "201" ]; then
        print_success "$name sent successfully (HTTP $HTTP_CODE)"
        echo "$BODY" | jq '.' 2>/dev/null || echo "$BODY"
    else
        print_error "$name failed (HTTP $HTTP_CODE)"
        echo "$BODY"
        return 1
    fi

    echo ""
}

# Send firing alert
send_firing() {
    print_header "Sending Firing Alert (Critical)"
    send_alert "$SCRIPT_DIR/alertmanager-firing.json" "Firing alert"
    print_info "Expected: Incident created, Slack channel created (if configured)"
}

# Send resolved alert
send_resolved() {
    print_header "Sending Resolved Alert"
    send_alert "$SCRIPT_DIR/alertmanager-resolved.json" "Resolved alert"
    print_info "Expected: Alert updated with ended_at timestamp"
    print_info "Note: Incident is NOT auto-resolved (must be done manually)"
}

# Send warning alert
send_warning() {
    print_header "Sending Warning Alert"
    send_alert "$SCRIPT_DIR/alertmanager-warning.json" "Warning alert"
    print_info "Expected: Incident created (lower priority than critical)"
}

# Send info alert
send_info() {
    print_header "Sending Info Alert"
    send_alert "$SCRIPT_DIR/alertmanager-info.json" "Info alert"
    print_info "Expected: Alert stored, NO incident created"
}

# Send multiple alerts
send_multiple() {
    print_header "Sending Multiple Alerts"
    send_alert "$SCRIPT_DIR/alertmanager-multiple.json" "Multiple alerts"
    print_info "Expected: Multiple incidents created (one per critical/warning alert)"
}

# List incidents
list_incidents() {
    print_header "Current Incidents"

    INCIDENTS=$(curl -s "$OPENINCIDENT_URL/api/v1/incidents")
    COUNT=$(echo "$INCIDENTS" | jq '. | length' 2>/dev/null || echo "0")

    echo "Total incidents: $COUNT"
    echo ""

    if [ "$COUNT" -gt 0 ]; then
        echo "$INCIDENTS" | jq '.[] | {
            incident_number,
            title,
            status,
            severity,
            slack_channel_name,
            created_at
        }' 2>/dev/null || echo "$INCIDENTS"
    else
        print_info "No incidents found"
    fi

    echo ""
}

# Main script
main() {
    local command="${1:-all}"

    case "$command" in
        firing)
            check_health
            send_firing
            list_incidents
            ;;
        resolved)
            check_health
            send_resolved
            list_incidents
            ;;
        warning)
            check_health
            send_warning
            list_incidents
            ;;
        info)
            check_health
            send_info
            list_incidents
            ;;
        multiple)
            check_health
            send_multiple
            list_incidents
            ;;
        all)
            check_health
            send_firing
            sleep 1
            send_warning
            sleep 1
            send_info
            sleep 1
            send_multiple
            sleep 1
            list_incidents

            print_header "Summary"
            print_success "All test alerts sent!"
            print_info "Check your Slack workspace for incident channels (if configured)"
            print_info "View incidents: curl $OPENINCIDENT_URL/api/v1/incidents | jq"
            print_info "View UI: http://localhost:3000"
            ;;
        clean)
            print_header "Cleaning Up"
            print_info "To clean up test data, restart the database:"
            echo "  docker-compose down -v"
            echo "  docker-compose up -d"
            ;;
        help|--help|-h)
            echo "Usage: $0 [command]"
            echo ""
            echo "Commands:"
            echo "  firing      Send firing alert (creates incident)"
            echo "  resolved    Send resolved alert (updates alert, does not auto-resolve incident)"
            echo "  warning     Send warning alert (creates incident, lower priority)"
            echo "  info        Send info alert (stored but does not create incident)"
            echo "  multiple    Send multiple alerts at once"
            echo "  all         Send all test alerts (default)"
            echo "  clean       Show how to clean up test data"
            echo "  help        Show this help message"
            echo ""
            echo "Environment variables:"
            echo "  OPENINCIDENT_URL    OpenIncident URL (default: http://localhost:8080)"
            echo ""
            echo "Examples:"
            echo "  $0"
            echo "  $0 firing"
            echo "  OPENINCIDENT_URL=http://example.com:8080 $0 all"
            ;;
        *)
            print_error "Unknown command: $command"
            echo "Run '$0 help' for usage information"
            exit 1
            ;;
    esac
}

# Run main function
main "$@"
