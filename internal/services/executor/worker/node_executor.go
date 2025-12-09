package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"

	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type NodeExecutor struct {
	eventBus events.EventBus
	redis    *redis.Client
	logger   logger.Logger
	client   *http.Client
}

type NodeExecutionRequest struct {
	NodeID     string                 `json:"nodeId"`
	NodeType   string                 `json:"nodeType"`
	Parameters map[string]interface{} `json:"parameters"`
	InputData  map[string]interface{} `json:"inputData"`
}

type NodeExecutionResult struct {
	Success bool                   `json:"success"`
	Output  map[string]interface{} `json:"output"`
	Error   string                 `json:"error,omitempty"`
}

func NewNodeExecutor(eventBus events.EventBus, redis *redis.Client, logger logger.Logger) *NodeExecutor {
	return &NodeExecutor{
		eventBus: eventBus,
		redis:    redis,
		logger:   logger,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (e *NodeExecutor) Execute(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	e.logger.Info("Executing node",
		"nodeId", request.NodeID,
		"nodeType", request.NodeType,
	)

	switch request.NodeType {
	case "http-request":
		return e.executeHTTPRequest(ctx, request)
	case "database":
		return e.executeDatabaseQuery(ctx, request)
	case "email":
		return e.executeEmail(ctx, request)
	case "slack":
		return e.executeSlack(ctx, request)
	case "code":
		return e.executeCode(ctx, request)
	case "webhook":
		return e.executeWebhook(ctx, request)
	case "transform":
		return e.executeTransform(ctx, request)
	case "filter":
		return e.executeFilter(ctx, request)
	case "aggregate":
		return e.executeAggregate(ctx, request)
	default:
		return e.executeCustomNode(ctx, request)
	}
}

func (e *NodeExecutor) executeHTTPRequest(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	// Extract parameters
	url, _ := request.Parameters["url"].(string)
	method, _ := request.Parameters["method"].(string)
	headers, _ := request.Parameters["headers"].(map[string]interface{})
	body, _ := request.Parameters["body"].(interface{})

	if url == "" {
		return &NodeExecutionResult{
			Success: false,
			Error:   "URL is required",
		}, nil
	}

	if method == "" {
		method = "GET"
	}

	// Prepare request body
	var reqBody io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return &NodeExecutionResult{
				Success: false,
				Error:   fmt.Sprintf("Failed to marshal request body: %v", err),
			}, nil
		}
		reqBody = bytes.NewBuffer(jsonBody)
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, method, url, reqBody)
	if err != nil {
		return &NodeExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to create request: %v", err),
		}, nil
	}

	// Set headers
	for key, value := range headers {
		if strValue, ok := value.(string); ok {
			req.Header.Set(key, strValue)
		}
	}

	// Execute request
	resp, err := e.client.Do(req)
	if err != nil {
		return &NodeExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("Request failed: %v", err),
		}, nil
	}
	defer resp.Body.Close()

	// Read response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return &NodeExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("Failed to read response: %v", err),
		}, nil
	}

	// Parse response
	var responseData interface{}
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		// If not JSON, return as string
		responseData = string(respBody)
	}

	return &NodeExecutionResult{
		Success: true,
		Output: map[string]interface{}{
			"statusCode": resp.StatusCode,
			"headers":    resp.Header,
			"body":       responseData,
		},
	}, nil
}

func (e *NodeExecutor) executeDatabaseQuery(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	// Database query execution logic
	// This would connect to the specified database and execute the query
	
	query, _ := request.Parameters["query"].(string)
	dbType, _ := request.Parameters["type"].(string)
	
	e.logger.Info("Executing database query",
		"type", dbType,
		"query", query,
	)
	
	// Mock response for now
	return &NodeExecutionResult{
		Success: true,
		Output: map[string]interface{}{
			"rows": []map[string]interface{}{
				{"id": 1, "name": "Example"},
			},
			"rowCount": 1,
		},
	}, nil
}

func (e *NodeExecutor) executeEmail(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	to, _ := request.Parameters["to"].(string)
	subject, _ := request.Parameters["subject"].(string)
	body, _ := request.Parameters["body"].(string)
	
	e.logger.Info("Sending email",
		"to", to,
		"subject", subject,
	)
	
	// Email sending logic would go here
	// This would integrate with SendGrid, AWS SES, etc.
	
	return &NodeExecutionResult{
		Success: true,
		Output: map[string]interface{}{
			"messageId": "mock-message-id",
			"status":    "sent",
		},
	}, nil
}

func (e *NodeExecutor) executeSlack(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	channel, _ := request.Parameters["channel"].(string)
	message, _ := request.Parameters["message"].(string)
	
	e.logger.Info("Sending Slack message",
		"channel", channel,
	)
	
	// Slack API integration would go here
	
	return &NodeExecutionResult{
		Success: true,
		Output: map[string]interface{}{
			"messageId": "mock-slack-message-id",
			"timestamp": time.Now().Unix(),
		},
	}, nil
}

func (e *NodeExecutor) executeCode(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	language, _ := request.Parameters["language"].(string)
	code, _ := request.Parameters["code"].(string)
	
	e.logger.Info("Executing code",
		"language", language,
	)
	
	// In production, this would execute code in a secure sandbox
	// For now, we'll only support simple JavaScript execution
	
	if language == "javascript" {
		// Use a JavaScript engine or sandbox
		// This is a simplified example - DO NOT use in production without proper sandboxing
		return e.executeJavaScript(ctx, code, request.InputData)
	}
	
	return &NodeExecutionResult{
		Success: false,
		Error:   fmt.Sprintf("Unsupported language: %s", language),
	}, nil
}

func (e *NodeExecutor) executeJavaScript(ctx context.Context, code string, inputData map[string]interface{}) (*NodeExecutionResult, error) {
	// WARNING: This is a simplified example
	// In production, use a proper JavaScript sandbox like V8 isolates
	
	// Create a temporary file with the code
	// Execute it with Node.js in a restricted environment
	// Return the result
	
	// For safety, we'll just return mock data
	return &NodeExecutionResult{
		Success: true,
		Output: map[string]interface{}{
			"result": "Code executed (mock)",
			"input":  inputData,
		},
	}, nil
}

func (e *NodeExecutor) executeWebhook(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	// Webhook execution - similar to HTTP request but with webhook-specific logic
	return e.executeHTTPRequest(ctx, request)
}

func (e *NodeExecutor) executeTransform(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	// Data transformation logic
	transformType, _ := request.Parameters["type"].(string)
	
	switch transformType {
	case "map":
		return e.executeMap(ctx, request)
	case "filter":
		return e.executeFilter(ctx, request)
	case "reduce":
		return e.executeReduce(ctx, request)
	default:
		// Pass through
		return &NodeExecutionResult{
			Success: true,
			Output:  request.InputData,
		}, nil
	}
}

func (e *NodeExecutor) executeMap(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	// Map transformation
	mapping, _ := request.Parameters["mapping"].(map[string]interface{})
	
	output := make(map[string]interface{})
	for key, value := range mapping {
		if inputKey, ok := value.(string); ok {
			if inputValue, exists := request.InputData[inputKey]; exists {
				output[key] = inputValue
			}
		}
	}
	
	return &NodeExecutionResult{
		Success: true,
		Output:  output,
	}, nil
}

func (e *NodeExecutor) executeFilter(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	// Filter logic
	conditions, _ := request.Parameters["conditions"].([]interface{})
	
	// Simple filter implementation
	// In production, this would support complex conditions
	
	return &NodeExecutionResult{
		Success: true,
		Output:  request.InputData,
	}, nil
}

func (e *NodeExecutor) executeReduce(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	// Reduce/aggregate logic
	return &NodeExecutionResult{
		Success: true,
		Output: map[string]interface{}{
			"result": "reduced data",
		},
	}, nil
}

func (e *NodeExecutor) executeAggregate(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	// Aggregation logic
	aggregationType, _ := request.Parameters["type"].(string)
	
	switch aggregationType {
	case "sum", "avg", "min", "max", "count":
		// Perform aggregation
		return &NodeExecutionResult{
			Success: true,
			Output: map[string]interface{}{
				"result": 0,
				"type":   aggregationType,
			},
		}, nil
	default:
		return &NodeExecutionResult{
			Success: false,
			Error:   fmt.Sprintf("Unknown aggregation type: %s", aggregationType),
		}, nil
	}
}

func (e *NodeExecutor) executeCustomNode(ctx context.Context, request NodeExecutionRequest) (*NodeExecutionResult, error) {
	// For custom nodes, we'll check if there's a registered handler
	// This would integrate with a plugin system
	
	e.logger.Warn("Unknown node type, using passthrough",
		"nodeType", request.NodeType,
	)
	
	return &NodeExecutionResult{
		Success: true,
		Output:  request.InputData,
	}, nil
}

// Sandbox execution for untrusted code
func (e *NodeExecutor) executeInSandbox(ctx context.Context, language, code string, input map[string]interface{}) (map[string]interface{}, error) {
	// In production, this would:
	// 1. Create an isolated container or VM
	// 2. Set resource limits (CPU, memory, time)
	// 3. Disable network access if not needed
	// 4. Execute the code
	// 5. Return the result
	
	// For now, return mock data
	return map[string]interface{}{
		"status": "executed",
		"output": "sandbox execution result",
	}, nil
}
