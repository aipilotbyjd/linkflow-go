#!/bin/bash
# Test script for Workflow DAG Validation

API_URL="http://localhost:8083/api"
TOKEN="test-auth-token"  # Replace with actual token

echo "🧪 Testing Workflow DAG Validation"
echo "=================================="

# Test 1: Valid DAG workflow
echo -e "\n1. Testing valid DAG workflow..."
VALID_WORKFLOW=$(curl -s -X POST $API_URL/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Valid DAG Workflow",
    "description": "A workflow with valid DAG structure",
    "nodes": [
      {
        "id": "trigger",
        "name": "Start Trigger",
        "type": "trigger",
        "position": {"x": 100, "y": 100}
      },
      {
        "id": "process1",
        "name": "Process Step 1",
        "type": "action",
        "position": {"x": 300, "y": 100}
      },
      {
        "id": "condition",
        "name": "Check Condition",
        "type": "condition",
        "position": {"x": 500, "y": 100}
      },
      {
        "id": "process2a",
        "name": "Process Branch A",
        "type": "action",
        "position": {"x": 700, "y": 50}
      },
      {
        "id": "process2b",
        "name": "Process Branch B",
        "type": "action",
        "position": {"x": 700, "y": 150}
      },
      {
        "id": "merge",
        "name": "Merge Results",
        "type": "merge",
        "position": {"x": 900, "y": 100}
      },
      {
        "id": "final",
        "name": "Final Step",
        "type": "action",
        "position": {"x": 1100, "y": 100}
      }
    ],
    "connections": [
      {"id": "c1", "source": "trigger", "target": "process1"},
      {"id": "c2", "source": "process1", "target": "condition"},
      {"id": "c3", "source": "condition", "target": "process2a", "sourcePort": "true"},
      {"id": "c4", "source": "condition", "target": "process2b", "sourcePort": "false"},
      {"id": "c5", "source": "process2a", "target": "merge"},
      {"id": "c6", "source": "process2b", "target": "merge"},
      {"id": "c7", "source": "merge", "target": "final"}
    ]
  }')

WORKFLOW_ID=$(echo $VALID_WORKFLOW | jq -r '.id')
echo "✅ Valid DAG workflow created with ID: $WORKFLOW_ID"

# Validate the workflow
VALIDATION=$(curl -s -X GET $API_URL/workflows/$WORKFLOW_ID/validate \
  -H "Authorization: Bearer $TOKEN")
IS_VALID=$(echo $VALIDATION | jq -r '.valid')
echo "   Validation result: valid=$IS_VALID"

# Test 2: Workflow with cycle (should fail)
echo -e "\n2. Testing workflow with cycle..."
CYCLE_WORKFLOW=$(curl -s -X POST $API_URL/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Workflow with Cycle",
    "description": "This workflow contains a cycle",
    "nodes": [
      {
        "id": "trigger",
        "name": "Start",
        "type": "trigger",
        "position": {"x": 100, "y": 100}
      },
      {
        "id": "node1",
        "name": "Node 1",
        "type": "action",
        "position": {"x": 300, "y": 100}
      },
      {
        "id": "node2",
        "name": "Node 2",
        "type": "action",
        "position": {"x": 500, "y": 100}
      },
      {
        "id": "node3",
        "name": "Node 3",
        "type": "action",
        "position": {"x": 700, "y": 100}
      }
    ],
    "connections": [
      {"id": "c1", "source": "trigger", "target": "node1"},
      {"id": "c2", "source": "node1", "target": "node2"},
      {"id": "c3", "source": "node2", "target": "node3"},
      {"id": "c4", "source": "node3", "target": "node1"}
    ]
  }')

if echo $CYCLE_WORKFLOW | jq -e '.error' > /dev/null; then
  echo "✅ Correctly rejected workflow with cycle"
else
  echo "❌ Failed to detect cycle in workflow"
fi

# Test 3: Workflow without trigger node (should fail)
echo -e "\n3. Testing workflow without trigger node..."
NO_TRIGGER=$(curl -s -X POST $API_URL/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "No Trigger Workflow",
    "description": "Missing trigger node",
    "nodes": [
      {
        "id": "action1",
        "name": "Action 1",
        "type": "action",
        "position": {"x": 100, "y": 100}
      },
      {
        "id": "action2",
        "name": "Action 2",
        "type": "action",
        "position": {"x": 300, "y": 100}
      }
    ],
    "connections": [
      {"id": "c1", "source": "action1", "target": "action2"}
    ]
  }')

if echo $NO_TRIGGER | jq -e '.error' > /dev/null; then
  echo "✅ Correctly rejected workflow without trigger node"
else
  echo "❌ Failed to detect missing trigger node"
fi

# Test 4: Workflow with orphaned nodes
echo -e "\n4. Testing workflow with orphaned nodes..."
ORPHAN_WORKFLOW=$(curl -s -X POST $API_URL/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Workflow with Orphans",
    "description": "Contains orphaned nodes",
    "nodes": [
      {
        "id": "trigger",
        "name": "Start",
        "type": "trigger",
        "position": {"x": 100, "y": 100}
      },
      {
        "id": "connected",
        "name": "Connected Node",
        "type": "action",
        "position": {"x": 300, "y": 100}
      },
      {
        "id": "orphan",
        "name": "Orphaned Node",
        "type": "action",
        "position": {"x": 500, "y": 200}
      }
    ],
    "connections": [
      {"id": "c1", "source": "trigger", "target": "connected"}
    ]
  }')

ORPHAN_ID=$(echo $ORPHAN_WORKFLOW | jq -r '.id')
if [ "$ORPHAN_ID" != "null" ]; then
  # Validate to check for warnings
  ORPHAN_VALIDATION=$(curl -s -X GET $API_URL/workflows/$ORPHAN_ID/validate \
    -H "Authorization: Bearer $TOKEN")
  WARNINGS=$(echo $ORPHAN_VALIDATION | jq -r '.warnings | length')
  if [ "$WARNINGS" -gt 0 ]; then
    echo "✅ Detected orphaned nodes (warnings: $WARNINGS)"
  else
    echo "⚠️  No warnings for orphaned nodes"
  fi
fi

# Test 5: Complex valid DAG with multiple paths
echo -e "\n5. Testing complex DAG with multiple execution paths..."
COMPLEX_WORKFLOW=$(curl -s -X POST $API_URL/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Complex Multi-Path Workflow",
    "description": "Complex workflow with splits, merges, and conditions",
    "nodes": [
      {
        "id": "webhook",
        "name": "Webhook Trigger",
        "type": "webhook",
        "position": {"x": 100, "y": 200},
        "parameters": {"path": "/api/trigger"}
      },
      {
        "id": "validate",
        "name": "Validate Input",
        "type": "action",
        "position": {"x": 300, "y": 200}
      },
      {
        "id": "split",
        "name": "Split Processing",
        "type": "split",
        "position": {"x": 500, "y": 200}
      },
      {
        "id": "process_a",
        "name": "Process Path A",
        "type": "action",
        "position": {"x": 700, "y": 100}
      },
      {
        "id": "process_b",
        "name": "Process Path B",
        "type": "action",
        "position": {"x": 700, "y": 300}
      },
      {
        "id": "condition_a",
        "name": "Check Result A",
        "type": "condition",
        "position": {"x": 900, "y": 100}
      },
      {
        "id": "success_a",
        "name": "Success Handler A",
        "type": "action",
        "position": {"x": 1100, "y": 50}
      },
      {
        "id": "failure_a",
        "name": "Failure Handler A",
        "type": "action",
        "position": {"x": 1100, "y": 150}
      },
      {
        "id": "merge",
        "name": "Merge Results",
        "type": "merge",
        "position": {"x": 1300, "y": 200}
      },
      {
        "id": "finalize",
        "name": "Finalize",
        "type": "action",
        "position": {"x": 1500, "y": 200}
      }
    ],
    "connections": [
      {"id": "c1", "source": "webhook", "target": "validate"},
      {"id": "c2", "source": "validate", "target": "split"},
      {"id": "c3", "source": "split", "target": "process_a", "sourcePort": "output"},
      {"id": "c4", "source": "split", "target": "process_b", "sourcePort": "output"},
      {"id": "c5", "source": "process_a", "target": "condition_a"},
      {"id": "c6", "source": "condition_a", "target": "success_a", "sourcePort": "true"},
      {"id": "c7", "source": "condition_a", "target": "failure_a", "sourcePort": "false"},
      {"id": "c8", "source": "success_a", "target": "merge"},
      {"id": "c9", "source": "failure_a", "target": "merge"},
      {"id": "c10", "source": "process_b", "target": "merge"},
      {"id": "c11", "source": "merge", "target": "finalize"}
    ]
  }')

COMPLEX_ID=$(echo $COMPLEX_WORKFLOW | jq -r '.id')
if [ "$COMPLEX_ID" != "null" ]; then
  echo "✅ Complex DAG workflow created with ID: $COMPLEX_ID"
  
  # Validate complex workflow
  COMPLEX_VALIDATION=$(curl -s -X GET $API_URL/workflows/$COMPLEX_ID/validate \
    -H "Authorization: Bearer $TOKEN")
  IS_VALID=$(echo $COMPLEX_VALIDATION | jq -r '.valid')
  ERRORS=$(echo $COMPLEX_VALIDATION | jq -r '.errors | length')
  WARNINGS=$(echo $COMPLEX_VALIDATION | jq -r '.warnings | length')
  echo "   Validation: valid=$IS_VALID, errors=$ERRORS, warnings=$WARNINGS"
fi

# Test 6: Invalid node connections
echo -e "\n6. Testing invalid node connections..."
INVALID_CONN=$(curl -s -X POST $API_URL/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Invalid Connections",
    "description": "References non-existent nodes",
    "nodes": [
      {
        "id": "trigger",
        "name": "Start",
        "type": "trigger",
        "position": {"x": 100, "y": 100}
      },
      {
        "id": "action",
        "name": "Action",
        "type": "action",
        "position": {"x": 300, "y": 100}
      }
    ],
    "connections": [
      {"id": "c1", "source": "trigger", "target": "action"},
      {"id": "c2", "source": "action", "target": "nonexistent"}
    ]
  }')

if echo $INVALID_CONN | jq -e '.error' > /dev/null; then
  echo "✅ Correctly rejected workflow with invalid connections"
else
  echo "❌ Failed to detect invalid connections"
fi

# Test 7: Merge node validation
echo -e "\n7. Testing merge node requirements..."
INVALID_MERGE=$(curl -s -X POST $API_URL/workflows \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Invalid Merge Node",
    "description": "Merge node with insufficient inputs",
    "nodes": [
      {
        "id": "trigger",
        "name": "Start",
        "type": "trigger",
        "position": {"x": 100, "y": 100}
      },
      {
        "id": "action",
        "name": "Single Action",
        "type": "action",
        "position": {"x": 300, "y": 100}
      },
      {
        "id": "merge",
        "name": "Merge",
        "type": "merge",
        "position": {"x": 500, "y": 100}
      },
      {
        "id": "final",
        "name": "Final",
        "type": "action",
        "position": {"x": 700, "y": 100}
      }
    ],
    "connections": [
      {"id": "c1", "source": "trigger", "target": "action"},
      {"id": "c2", "source": "action", "target": "merge"},
      {"id": "c3", "source": "merge", "target": "final"}
    ]
  }')

MERGE_ID=$(echo $INVALID_MERGE | jq -r '.id')
if [ "$MERGE_ID" != "null" ]; then
  MERGE_VALIDATION=$(curl -s -X GET $API_URL/workflows/$MERGE_ID/validate \
    -H "Authorization: Bearer $TOKEN")
  WARNINGS=$(echo $MERGE_VALIDATION | jq -r '.warnings | length')
  if [ "$WARNINGS" -gt 0 ]; then
    echo "✅ Detected merge node with insufficient inputs"
  else
    echo "⚠️  No warning for merge node with single input"
  fi
fi

echo -e "\n=================================="
echo "✅ DAG validation tests completed!"
