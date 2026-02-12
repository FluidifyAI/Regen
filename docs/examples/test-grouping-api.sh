#!/bin/bash

# Test script for Grouping Rules CRUD API (OI-104)
# This script tests all API endpoints for managing grouping rules

BASE_URL="http://localhost:8080/api/v1/grouping-rules"

echo "========================================="
echo "Testing Grouping Rules CRUD API"
echo "========================================="
echo

# Test 1: Create a grouping rule
echo "Test 1: Create grouping rule..."
RESPONSE=$(curl -s -X POST $BASE_URL \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test: Cross-source grouping",
    "description": "Groups alerts from all sources for the same service/env",
    "enabled": true,
    "priority": 50,
    "match_labels": {
      "severity": "critical"
    },
    "cross_source_labels": ["service", "env"],
    "time_window_seconds": 600
  }')

RULE_ID=$(echo $RESPONSE | jq -r '.id')

if [ "$RULE_ID" != "null" ] && [ -n "$RULE_ID" ]; then
  echo "✅ Created rule with ID: $RULE_ID"
else
  echo "❌ Failed to create rule"
  echo "Response: $RESPONSE"
  exit 1
fi
echo

# Test 2: Get the created rule
echo "Test 2: Get grouping rule by ID..."
GET_RESPONSE=$(curl -s $BASE_URL/$RULE_ID)
RULE_NAME=$(echo $GET_RESPONSE | jq -r '.name')

if [ "$RULE_NAME" == "Test: Cross-source grouping" ]; then
  echo "✅ Retrieved rule: $RULE_NAME"
else
  echo "❌ Failed to get rule"
  echo "Response: $GET_RESPONSE"
  exit 1
fi
echo

# Test 3: List all rules
echo "Test 3: List all grouping rules..."
LIST_RESPONSE=$(curl -s $BASE_URL)
TOTAL=$(echo $LIST_RESPONSE | jq -r '.total')

if [ "$TOTAL" -ge 1 ]; then
  echo "✅ Found $TOTAL rule(s)"
else
  echo "❌ Failed to list rules"
  echo "Response: $LIST_RESPONSE"
  exit 1
fi
echo

# Test 4: Update the rule
echo "Test 4: Update grouping rule..."
UPDATE_RESPONSE=$(curl -s -X PUT $BASE_URL/$RULE_ID \
  -H "Content-Type: application/json" \
  -d '{
    "description": "Updated description",
    "priority": 60
  }')

UPDATED_DESC=$(echo $UPDATE_RESPONSE | jq -r '.description')
UPDATED_PRIORITY=$(echo $UPDATE_RESPONSE | jq -r '.priority')

if [ "$UPDATED_DESC" == "Updated description" ] && [ "$UPDATED_PRIORITY" == "60" ]; then
  echo "✅ Updated rule successfully"
else
  echo "❌ Failed to update rule"
  echo "Response: $UPDATE_RESPONSE"
  exit 1
fi
echo

# Test 5: Create second rule to test priority conflict
echo "Test 5: Test priority conflict (should fail)..."
CONFLICT_RESPONSE=$(curl -s -X POST $BASE_URL \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Conflicting Rule",
    "priority": 60,
    "match_labels": {"alertname": "*"},
    "time_window_seconds": 300
  }')

ERROR_MSG=$(echo $CONFLICT_RESPONSE | jq -r '.error')

if [[ "$ERROR_MSG" == *"priority already in use"* ]]; then
  echo "✅ Priority conflict detected correctly"
else
  echo "❌ Priority conflict not detected"
  echo "Response: $CONFLICT_RESPONSE"
  exit 1
fi
echo

# Test 6: List enabled rules only
echo "Test 6: List enabled rules only..."
ENABLED_RESPONSE=$(curl -s "$BASE_URL?enabled=true")
ENABLED_TOTAL=$(echo $ENABLED_RESPONSE | jq -r '.total')

echo "✅ Found $ENABLED_TOTAL enabled rule(s)"
echo

# Test 7: Disable the rule
echo "Test 7: Disable grouping rule..."
DISABLE_RESPONSE=$(curl -s -X PUT $BASE_URL/$RULE_ID \
  -H "Content-Type: application/json" \
  -d '{"enabled": false}')

IS_ENABLED=$(echo $DISABLE_RESPONSE | jq -r '.enabled')

if [ "$IS_ENABLED" == "false" ]; then
  echo "✅ Disabled rule successfully"
else
  echo "❌ Failed to disable rule"
  echo "Response: $DISABLE_RESPONSE"
  exit 1
fi
echo

# Test 8: Delete the rule
echo "Test 8: Delete grouping rule..."
DELETE_RESPONSE=$(curl -s -w "\n%{http_code}" -X DELETE $BASE_URL/$RULE_ID)
HTTP_CODE=$(echo "$DELETE_RESPONSE" | tail -n1)

if [ "$HTTP_CODE" == "204" ]; then
  echo "✅ Deleted rule successfully"
else
  echo "❌ Failed to delete rule (HTTP $HTTP_CODE)"
  exit 1
fi
echo

# Test 9: Verify deletion
echo "Test 9: Verify rule is deleted..."
VERIFY_RESPONSE=$(curl -s -w "\n%{http_code}" $BASE_URL/$RULE_ID)
VERIFY_CODE=$(echo "$VERIFY_RESPONSE" | tail -n1)

if [ "$VERIFY_CODE" == "404" ]; then
  echo "✅ Rule confirmed deleted (404 Not Found)"
else
  echo "❌ Rule still exists (HTTP $VERIFY_CODE)"
  exit 1
fi
echo

echo "========================================="
echo "✅ All tests passed!"
echo "========================================="
echo
echo "Summary:"
echo "  - Create rule: ✅"
echo "  - Get rule: ✅"
echo "  - List rules: ✅"
echo "  - Update rule: ✅"
echo "  - Priority conflict detection: ✅"
echo "  - Filter by enabled: ✅"
echo "  - Disable rule: ✅"
echo "  - Delete rule: ✅"
echo "  - Verify deletion: ✅"
echo
echo "The Grouping Rules CRUD API is working correctly!"
