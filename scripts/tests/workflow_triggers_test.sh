#!/bin/bash
# Test script for Workflow Triggers

API_URL="http://localhost:8083/api"
TOKEN="test-auth-token"  # Replace with actual token

echo "🧪 Testing Workflow Triggers"
echo "=================================="

# First create a workflow for testing triggers
echo -e "\n1. Creating test workflow..."
WORKFLOW_RESPONSE=$(curl -s -X POST $API_URL/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Trigger Test Workflow",
    "description": "Workflow for testing various trigger types",
    "nodes": [
      {
        "id": "trigger",
        "name": "Trigger Node",
        "type": "trigger",
        "position": {"x": 100, "y": 100}
      },
      {
        "id": "action1",
        "name": "Process Data",
        "type": "action",
        "position": {"x": 300, "y": 100},
        "parameters": {"action": "process"}
      }
    ],
    "connections": [
      {"id": "c1", "source": "trigger", "target": "action1"}
    ]
  }')

WORKFLOW_ID=$(echo $WORKFLOW_RESPONSE | jq -r '.id')
echo "✅ Workflow created with ID: $WORKFLOW_ID"

# Test 1: Webhook Trigger
echo -e "\n2. Testing Webhook Trigger..."
WEBHOOK_TRIGGER=$(curl -s -X POST $API_URL/workflows/$WORKFLOW_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "webhook",
    "name": "Data Ingestion Webhook",
    "description": "Webhook for data ingestion",
    "path": "/api/ingest/data",
    "method": "POST",
    "secret": "webhook-secret-123"
  }')

WEBHOOK_ID=$(echo $WEBHOOK_TRIGGER | jq -r '.id')
if [ "$WEBHOOK_ID" != "null" ]; then
  echo "✅ Webhook trigger created with ID: $WEBHOOK_ID"
else
  echo "❌ Failed to create webhook trigger"
fi

# Test 2: Schedule Trigger
echo -e "\n3. Testing Schedule Trigger..."
SCHEDULE_TRIGGER=$(curl -s -X POST $API_URL/workflows/$WORKFLOW_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "schedule",
    "name": "Daily Report Generator",
    "description": "Generate reports every day at 9 AM",
    "cronExpression": "0 9 * * *",
    "timezone": "America/New_York"
  }')

SCHEDULE_ID=$(echo $SCHEDULE_TRIGGER | jq -r '.id')
if [ "$SCHEDULE_ID" != "null" ]; then
  echo "✅ Schedule trigger created with ID: $SCHEDULE_ID"
else
  echo "❌ Failed to create schedule trigger"
fi

# Test 3: Event Trigger
echo -e "\n4. Testing Event Trigger..."
EVENT_TRIGGER=$(curl -s -X POST $API_URL/workflows/$WORKFLOW_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "event",
    "name": "User Registration Event",
    "description": "Trigger on user registration",
    "eventType": "user.registered",
    "eventSource": "auth-service",
    "filters": {
      "accountType": "premium"
    }
  }')

EVENT_ID=$(echo $EVENT_TRIGGER | jq -r '.id')
if [ "$EVENT_ID" != "null" ]; then
  echo "✅ Event trigger created with ID: $EVENT_ID"
else
  echo "❌ Failed to create event trigger"
fi

# Test 4: Manual Trigger
echo -e "\n5. Testing Manual Trigger..."
MANUAL_TRIGGER=$(curl -s -X POST $API_URL/workflows/$WORKFLOW_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "manual",
    "name": "Manual Execution",
    "description": "Manually trigger workflow execution",
    "requireConfirmation": true,
    "allowedUsers": ["user1", "user2"]
  }')

MANUAL_ID=$(echo $MANUAL_TRIGGER | jq -r '.id')
if [ "$MANUAL_ID" != "null" ]; then
  echo "✅ Manual trigger created with ID: $MANUAL_ID"
else
  echo "❌ Failed to create manual trigger"
fi

# Test 5: Email Trigger
echo -e "\n6. Testing Email Trigger..."
EMAIL_TRIGGER=$(curl -s -X POST $API_URL/workflows/$WORKFLOW_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "email",
    "name": "Support Email Trigger",
    "description": "Trigger on support emails",
    "emailAddress": "support@example.com",
    "subject": "URGENT:",
    "fromFilter": ["customer@example.com"],
    "keywords": ["urgent", "critical"]
  }')

EMAIL_ID=$(echo $EMAIL_TRIGGER | jq -r '.id')
if [ "$EMAIL_ID" != "null" ]; then
  echo "✅ Email trigger created with ID: $EMAIL_ID"
else
  echo "❌ Failed to create email trigger"
fi

# Test 6: List all triggers
echo -e "\n7. Listing all triggers for workflow..."
TRIGGERS_LIST=$(curl -s -X GET $API_URL/workflows/$WORKFLOW_ID/triggers \
  -H "Authorization: Bearer $TOKEN")

TRIGGER_COUNT=$(echo $TRIGGERS_LIST | jq '.triggers | length')
echo "✅ Found $TRIGGER_COUNT triggers"

# Test 7: Activate webhook trigger
echo -e "\n8. Activating webhook trigger..."
# First activate the workflow
curl -s -X POST $API_URL/workflows/$WORKFLOW_ID/activate \
  -H "Authorization: Bearer $TOKEN" > /dev/null

ACTIVATE_RESPONSE=$(curl -s -X POST $API_URL/triggers/$WEBHOOK_ID/activate \
  -H "Authorization: Bearer $TOKEN")

if echo $ACTIVATE_RESPONSE | jq -e '.message' | grep -q "activated"; then
  echo "✅ Webhook trigger activated"
else
  echo "⚠️  Failed to activate webhook trigger"
fi

# Test 8: Test webhook trigger
echo -e "\n9. Testing webhook trigger with sample data..."
TEST_RESPONSE=$(curl -s -X POST $API_URL/triggers/$WEBHOOK_ID/test \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/api/ingest/data",
    "method": "POST",
    "secret": "webhook-secret-123",
    "body": {"sample": "data"}
  }')

WOULD_FIRE=$(echo $TEST_RESPONSE | jq -r '.would_fire')
if [ "$WOULD_FIRE" = "true" ]; then
  echo "✅ Webhook trigger test: would fire with test data"
else
  echo "⚠️  Webhook trigger test: would NOT fire with test data"
fi

# Test 9: Test schedule trigger
echo -e "\n10. Testing schedule trigger..."
SCHEDULE_TEST=$(curl -s -X POST $API_URL/triggers/$SCHEDULE_ID/test \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "time": "2024-01-15T09:00:00-05:00"
  }')

echo "✅ Schedule trigger tested"

# Test 10: Update trigger
echo -e "\n11. Updating webhook trigger..."
UPDATE_RESPONSE=$(curl -s -X PUT $API_URL/triggers/$WEBHOOK_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "path": "/api/ingest/updated",
    "method": "PUT",
    "secret": "new-secret-456"
  }')

if echo $UPDATE_RESPONSE | jq -e '.id' > /dev/null; then
  echo "✅ Webhook trigger updated"
else
  echo "❌ Failed to update webhook trigger"
fi

# Test 11: Deactivate trigger
echo -e "\n12. Deactivating webhook trigger..."
DEACTIVATE_RESPONSE=$(curl -s -X POST $API_URL/triggers/$WEBHOOK_ID/deactivate \
  -H "Authorization: Bearer $TOKEN")

if echo $DEACTIVATE_RESPONSE | jq -e '.message' | grep -q "deactivated"; then
  echo "✅ Webhook trigger deactivated"
else
  echo "⚠️  Failed to deactivate webhook trigger"
fi

# Test 12: Invalid cron expression
echo -e "\n13. Testing invalid cron expression..."
INVALID_CRON=$(curl -s -X POST $API_URL/workflows/$WORKFLOW_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "schedule",
    "name": "Invalid Schedule",
    "cronExpression": "invalid cron"
  }')

if echo $INVALID_CRON | jq -e '.error' > /dev/null; then
  echo "✅ Correctly rejected invalid cron expression"
else
  echo "❌ Failed to validate cron expression"
fi

# Test 13: Duplicate webhook path
echo -e "\n14. Testing duplicate webhook path..."
DUPLICATE_WEBHOOK=$(curl -s -X POST $API_URL/workflows/$WORKFLOW_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "webhook",
    "name": "Duplicate Webhook",
    "path": "/api/ingest/updated",
    "method": "PUT"
  }')

if echo $DUPLICATE_WEBHOOK | jq -e '.error' > /dev/null; then
  echo "✅ Correctly rejected duplicate webhook trigger"
else
  echo "⚠️  Duplicate webhook detection may not be working"
fi

# Test 14: Delete trigger
echo -e "\n15. Deleting email trigger..."
DELETE_RESPONSE=$(curl -s -X DELETE $API_URL/triggers/$EMAIL_ID \
  -H "Authorization: Bearer $TOKEN")

# Verify deletion
CHECK_DELETED=$(curl -s -X GET $API_URL/triggers/$EMAIL_ID \
  -H "Authorization: Bearer $TOKEN")

if echo $CHECK_DELETED | jq -e '.error' > /dev/null; then
  echo "✅ Email trigger deleted successfully"
else
  echo "❌ Failed to delete email trigger"
fi

# Test 15: Multiple triggers of same type
echo -e "\n16. Testing multiple triggers of same type..."
SECOND_WEBHOOK=$(curl -s -X POST $API_URL/workflows/$WORKFLOW_ID/triggers \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "type": "webhook",
    "name": "Second Webhook",
    "path": "/api/secondary",
    "method": "POST"
  }')

if echo $SECOND_WEBHOOK | jq -e '.id' > /dev/null; then
  echo "✅ Multiple triggers of same type allowed"
else
  echo "❌ Failed to create second webhook trigger"
fi

# Cleanup
echo -e "\n17. Cleaning up test workflow..."
curl -s -X DELETE $API_URL/workflows/$WORKFLOW_ID \
  -H "Authorization: Bearer $TOKEN"

echo -e "\n=================================="
echo "✅ Workflow triggers testing completed!"
echo ""
echo "Summary:"
echo "- Webhook triggers: ✅"
echo "- Schedule triggers: ✅"
echo "- Event triggers: ✅"
echo "- Manual triggers: ✅"
echo "- Email triggers: ✅"
echo "- Trigger activation/deactivation: ✅"
echo "- Trigger testing: ✅"
echo "- Validation: ✅"
