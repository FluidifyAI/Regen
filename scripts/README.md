# OpenIncident Test Scripts

This directory contains test scripts for validating OpenIncident functionality.

## E2E Test Script

**`e2e-test.sh`** - End-to-end test suite that validates the complete alert-to-incident workflow.

### What It Tests

1. **Health & Readiness**
   - `/health` endpoint returns `200 OK`
   - `/ready` endpoint confirms database and Redis connectivity

2. **Webhook Ingestion**
   - Prometheus webhook accepts alerts
   - Alerts are stored in database
   - Critical alerts auto-create incidents

3. **Incident API**
   - List incidents with pagination
   - Get incident by UUID
   - Get incident by incident number
   - Update incident status (valid transitions)
   - Reject invalid status transitions
   - Update incident severity
   - Filter incidents by status

4. **Data Validation**
   - Incident fields are correctly populated from alert
   - Timestamps are set properly
   - Created by system/alertmanager tracking

### Usage

```bash
# Run full test suite
./scripts/e2e-test.sh

# Run with detailed output
./scripts/e2e-test.sh --verbose

# Only cleanup test data
./scripts/e2e-test.sh --cleanup-only

# Show help
./scripts/e2e-test.sh --help
```

### Environment Variables

- `OPENINCIDENT_URL` - OpenIncident base URL (default: `http://localhost:8080`)

### Example Output

```
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
OpenIncident E2E Test Suite
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Target: http://localhost:8080
  Mode:   Standard

▶ Test 1: Health Endpoint
  ✓ Health endpoint responds (HTTP 200)
  ✓ Health status is 'ok'

▶ Test 2: Readiness Endpoint
  ✓ Ready endpoint responds (HTTP 200)
  ✓ Status is 'ready'
  ✓ Database is ready
  ✓ Redis is ready

▶ Test 3: Prometheus Webhook Ingestion
  ✓ Webhook accepted (HTTP 200)
  ✓ 1 alert received
  ✓ 1 incident auto-created

...

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
Test Summary
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Total Tests:  33
  Passed:       29
  Failed:       4

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✗ SOME TESTS FAILED
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

**Note:** 4 tests currently fail due to a known transaction handling bug in the service layer (documented in `backend/internal/api/handlers/incidents_test.go` with FIXME comments). The service creates transactions but repositories don't use the transaction context, causing incident status updates to be logged but not persisted. This is a pre-existing architectural issue that will be fixed in a future refactoring. The E2E test successfully identified this bug!

### Prerequisites

- OpenIncident backend running (`make dev` or `docker-compose up`)
- `curl` installed
- `jq` installed (for JSON parsing)

### Exit Codes

- `0` - All tests passed
- `1` - One or more tests failed

### Integration with CI/CD

This script is designed to be run in CI/CD pipelines:

```yaml
# GitHub Actions example
- name: Run E2E Tests
  run: |
    docker-compose up -d
    sleep 5  # Wait for services to start
    ./scripts/e2e-test.sh
  env:
    OPENINCIDENT_URL: http://localhost:8080
```

### Test Coverage

The E2E test validates:

- ✅ HTTP endpoints return correct status codes
- ✅ Response JSON structure matches API spec
- ✅ Database persistence (incidents, alerts)
- ✅ Business logic (auto-incident creation)
- ✅ State machine validation (status transitions)
- ✅ Query parameters (pagination, filters)
- ✅ Error handling (invalid transitions, bad requests)

### Known Limitations

- **No Slack Integration Testing**: Slack channel creation is tested separately
- **No Timeline Verification**: Timeline entries are created but not validated in detail
- **No Multi-User Testing**: All operations use system/alertmanager as actor
- **No Concurrent Request Testing**: Tests run sequentially

### Troubleshooting

**OpenIncident is not running**
```bash
make dev
# OR
docker-compose up -d
```

**Database not ready**
```bash
# Check PostgreSQL is running
docker-compose ps postgres

# Check logs
docker-compose logs postgres
```

**Tests fail intermittently**
```bash
# Increase startup wait time
sleep 10
./scripts/e2e-test.sh
```

**jq command not found**
```bash
# macOS
brew install jq

# Ubuntu/Debian
sudo apt-get install jq

# Alpine
apk add jq
```

### Adding New Tests

To add a new test to the suite:

1. Create a test function:
   ```bash
   test_my_new_feature() {
       print_section "Test X: My New Feature"

       # Make API call
       RESPONSE=$(curl -s -w "\n%{http_code}" "$OPENINCIDENT_URL/api/v1/...")
       HTTP_CODE=$(echo "$RESPONSE" | tail -n 1)
       BODY=$(echo "$RESPONSE" | sed '$d')

       # Assert expectations
       assert_http_status "200" "$HTTP_CODE" "My feature works"

       VALUE=$(echo "$BODY" | jq -r '.field' 2>/dev/null || echo "")
       assert_equals "expected" "$VALUE" "Field has correct value"
   }
   ```

2. Add to main execution:
   ```bash
   main() {
       # ... existing tests ...
       test_my_new_feature
       # ...
   }
   ```

3. Update test count expectations in this README

### Related Scripts

- **`docs/examples/test-alerts.sh`** - Manual alert testing with various payloads
- Integration tests in `backend/internal/api/handlers/*_test.go`
- Unit tests in `backend/internal/services/*_test.go`
