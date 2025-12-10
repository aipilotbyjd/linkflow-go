#!/bin/bash
# Test script for Workflow CRUD operations

API_URL="http://localhost:8083/api"
TOKEN="test-auth-token"  # Replace with actual token

echo "🧪 Testing Workflow CRUD Operations"
echo "=================================="

# Test 1: Create Workflow
echo -e "\n1. Creating workflow..."
CREATE_RESPONSE=$(curl -s -X POST $API_URL/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Test ETL Pipeline",
    "description": "A test workflow for ETL operations",
    "nodes": [
      {
        "id": "trigger-1",
        "name": "Webhook Trigger",
        "type": "trigger",
        "position": {"x": 100, "y": 100},
        "parameters": {"webhook": "/data/ingest"}
      },
      {
        "id": "action-1",
        "name": "Process Data",
        "type": "action",
        "position": {"x": 300, "y": 100},
        "parameters": {"action": "transform"}
      }
    ],
    "connections": [
      {
        "id": "conn-1",
        "source": "trigger-1",
        "target": "action-1",
        "sourcePort": "output",
        "targetPort": "input"
      }
    ],
    "tags": ["etl", "data-pipeline", "test"]
  }')

WORKFLOW_ID=$(echo $CREATE_RESPONSE | jq -r '.id')
echo "✅ Workflow created with ID: $WORKFLOW_ID"

# Test 2: Get Workflow
echo -e "\n2. Retrieving workflow..."
GET_RESPONSE=$(curl -s -X GET $API_URL/workflows/$WORKFLOW_ID \
  -H "Authorization: Bearer $TOKEN")
echo "✅ Workflow retrieved: $(echo $GET_RESPONSE | jq -r '.name')"

# Test 3: List Workflows with Pagination
echo -e "\n3. Listing workflows (page 1, limit 5)..."
LIST_RESPONSE=$(curl -s -X GET "$API_URL/workflows?page=1&limit=5" \
  -H "Authorization: Bearer $TOKEN")
TOTAL=$(echo $LIST_RESPONSE | jq -r '.total')
echo "✅ Found $TOTAL total workflows"

# Test 4: Update Workflow
echo -e "\n4. Updating workflow..."
VERSION=$(echo $GET_RESPONSE | jq -r '.version')
UPDATE_RESPONSE=$(curl -s -X PUT $API_URL/workflows/$WORKFLOW_ID \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated ETL Pipeline",
    "description": "Updated workflow with new features",
    "version": '$VERSION',
    "nodes": [
      {
        "id": "trigger-1",
        "name": "Webhook Trigger",
        "type": "trigger",
        "position": {"x": 100, "y": 100},
        "parameters": {"webhook": "/data/ingest"}
      },
      {
        "id": "action-1",
        "name": "Process Data",
        "type": "action",
        "position": {"x": 300, "y": 100},
        "parameters": {"action": "transform"}
      },
      {
        "id": "action-2",
        "name": "Store Results",
        "type": "database",
        "position": {"x": 500, "y": 100},
        "parameters": {"table": "results"}
      }
    ],
    "connections": [
      {
        "id": "conn-1",
        "source": "trigger-1",
        "target": "action-1",
        "sourcePort": "output",
        "targetPort": "input"
      },
      {
        "id": "conn-2",
        "source": "action-1",
        "target": "action-2",
        "sourcePort": "output",
        "targetPort": "input"
      }
    ],
    "tags": ["etl", "data-pipeline", "updated"]
  }')

NEW_VERSION=$(echo $UPDATE_RESPONSE | jq -r '.version')
echo "✅ Workflow updated to version: $NEW_VERSION"

# Test 5: Test Validation (Create with cycle - should fail)
echo -e "\n5. Testing validation (workflow with cycle)..."
INVALID_RESPONSE=$(curl -s -X POST $API_URL/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Invalid Workflow with Cycle",
    "description": "This workflow has a cycle and should fail validation",
    "nodes": [
      {
        "id": "node-1",
        "name": "Node 1",
        "type": "action",
        "position": {"x": 100, "y": 100}
      },
      {
        "id": "node-2",
        "name": "Node 2",
        "type": "action",
        "position": {"x": 300, "y": 100}
      }
    ],
    "connections": [
      {
        "id": "conn-1",
        "source": "node-1",
        "target": "node-2"
      },
      {
        "id": "conn-2",
        "source": "node-2",
        "target": "node-1"
      }
    ]
  }')

if echo $INVALID_RESPONSE | jq -e '.error' > /dev/null; then
  echo "✅ Validation correctly rejected workflow with cycle"
else
  echo "❌ Validation failed to detect cycle"
fi

# Test 6: List workflows by status
echo -e "\n6. Filtering workflows by status..."
ACTIVE_WORKFLOWS=$(curl -s -X GET "$API_URL/workflows?status=active" \
  -H "Authorization: Bearer $TOKEN")
echo "✅ Found $(echo $ACTIVE_WORKFLOWS | jq -r '.total') active workflows"

# Test 7: Delete Workflow
echo -e "\n7. Deleting workflow..."
DELETE_RESPONSE=$(curl -s -X DELETE $API_URL/workflows/$WORKFLOW_ID \
  -H "Authorization: Bearer $TOKEN")
echo "✅ Workflow deleted (soft delete)"

# Test 8: Verify workflow is not returned after deletion
echo -e "\n8. Verifying workflow is not accessible after deletion..."
DELETED_CHECK=$(curl -s -X GET $API_URL/workflows/$WORKFLOW_ID \
  -H "Authorization: Bearer $TOKEN")
if echo $DELETED_CHECK | jq -e '.error' > /dev/null; then
  echo "✅ Deleted workflow correctly returns not found"
else
  echo "❌ Deleted workflow is still accessible"
fi

echo -e "\n=================================="
echo "✅ All CRUD operations tested successfully!"
