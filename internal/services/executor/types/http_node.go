package types

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/linkflow-go/pkg/logger"
)

// HTTPNodeExecutor handles HTTP request nodes
type HTTPNodeExecutor struct {
	client *http.Client
	logger logger.Logger
}

// HTTPNodeConfig represents configuration for HTTP nodes
type HTTPNodeConfig struct {
	Method          string            `json:"method"`
	URL             string            `json:"url"`
	Headers         map[string]string `json:"headers"`
	Body            interface{}       `json:"body"`
	QueryParams     map[string]string `json:"queryParams"`
	Authentication  AuthConfig        `json:"authentication"`
	Timeout         int               `json:"timeout"` // in seconds
	RetryOnFailure  bool              `json:"retryOnFailure"`
	MaxRetries      int               `json:"maxRetries"`
	FollowRedirects bool              `json:"followRedirects"`
	ValidateSSL     bool              `json:"validateSSL"`
}

// AuthConfig represents authentication configuration
type AuthConfig struct {
	Type         string `json:"type"` // none, basic, bearer, api-key
	Username     string `json:"username,omitempty"`
	Password     string `json:"password,omitempty"`
	Token        string `json:"token,omitempty"`
	APIKey       string `json:"apiKey,omitempty"`
	APIKeyHeader string `json:"apiKeyHeader,omitempty"`
}

// NewHTTPNodeExecutor creates a new HTTP node executor
func NewHTTPNodeExecutor(logger logger.Logger) *HTTPNodeExecutor {
	return &HTTPNodeExecutor{
		client: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		logger: logger,
	}
}

// Execute executes an HTTP request node
func (e *HTTPNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	config, err := e.parseConfig(node.Parameters)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Interpolate variables in URL
	url := e.interpolateVariables(config.URL, input)

	// Build request
	req, err := e.buildRequest(ctx, config, url, input)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	// Execute with retries
	var resp *http.Response
	var lastErr error

	maxAttempts := 1
	if config.RetryOnFailure && config.MaxRetries > 0 {
		maxAttempts = config.MaxRetries + 1
	}

	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			// Wait before retry
			time.Sleep(time.Duration(attempt) * time.Second)
			e.logger.Info("Retrying HTTP request", "attempt", attempt, "url", url)
		}

		resp, lastErr = e.client.Do(req)
		if lastErr == nil && resp.StatusCode < 500 {
			break // Success or client error (no retry needed)
		}

		if resp != nil {
			resp.Body.Close()
		}
	}

	if lastErr != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", lastErr)
	}
	defer resp.Body.Close()

	// Parse response
	return e.parseResponse(resp)
}

// ValidateInput validates the input for the HTTP node
func (e *HTTPNodeExecutor) ValidateInput(node Node, input map[string]interface{}) error {
	config, err := e.parseConfig(node.Parameters)
	if err != nil {
		return err
	}

	if config.URL == "" {
		return fmt.Errorf("URL is required")
	}

	if config.Method == "" {
		config.Method = "GET"
	}

	// Validate method
	validMethods := []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	valid := false
	for _, m := range validMethods {
		if strings.ToUpper(config.Method) == m {
			valid = true
			break
		}
	}

	if !valid {
		return fmt.Errorf("invalid HTTP method: %s", config.Method)
	}

	return nil
}

// GetTimeout returns the timeout for the HTTP request
func (e *HTTPNodeExecutor) GetTimeout() time.Duration {
	return 30 * time.Second
}

// parseConfig parses the node configuration
func (e *HTTPNodeExecutor) parseConfig(config interface{}) (*HTTPNodeConfig, error) {
	// Convert config to HTTPNodeConfig
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}

	var httpConfig HTTPNodeConfig
	if err := json.Unmarshal(jsonData, &httpConfig); err != nil {
		return nil, err
	}

	// Set defaults
	if httpConfig.Method == "" {
		httpConfig.Method = "GET"
	}

	if httpConfig.ValidateSSL == false {
		httpConfig.ValidateSSL = true
	}

	if httpConfig.FollowRedirects == false {
		httpConfig.FollowRedirects = true
	}

	return &httpConfig, nil
}

// buildRequest builds the HTTP request
func (e *HTTPNodeExecutor) buildRequest(ctx context.Context, config *HTTPNodeConfig, url string, input map[string]interface{}) (*http.Request, error) {
	// Prepare body
	var body io.Reader
	if config.Body != nil {
		// Interpolate variables in body
		bodyData := e.interpolateInObject(config.Body, input)

		jsonBody, err := json.Marshal(bodyData)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal body: %w", err)
		}
		body = bytes.NewReader(jsonBody)
	}

	// Create request
	req, err := http.NewRequestWithContext(ctx, strings.ToUpper(config.Method), url, body)
	if err != nil {
		return nil, err
	}

	// Add headers
	for key, value := range config.Headers {
		req.Header.Set(key, e.interpolateVariables(value, input))
	}

	// Set content type if not set and body exists
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	// Add query parameters
	if len(config.QueryParams) > 0 {
		q := req.URL.Query()
		for key, value := range config.QueryParams {
			q.Add(key, e.interpolateVariables(value, input))
		}
		req.URL.RawQuery = q.Encode()
	}

	// Add authentication
	e.addAuthentication(req, config.Authentication)

	return req, nil
}

// addAuthentication adds authentication to the request
func (e *HTTPNodeExecutor) addAuthentication(req *http.Request, auth AuthConfig) {
	switch auth.Type {
	case "basic":
		req.SetBasicAuth(auth.Username, auth.Password)
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+auth.Token)
	case "api-key":
		if auth.APIKeyHeader != "" {
			req.Header.Set(auth.APIKeyHeader, auth.APIKey)
		} else {
			req.Header.Set("X-API-Key", auth.APIKey)
		}
	}
}

// parseResponse parses the HTTP response
func (e *HTTPNodeExecutor) parseResponse(resp *http.Response) (map[string]interface{}, error) {
	// Read body
	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	result := map[string]interface{}{
		"statusCode": resp.StatusCode,
		"status":     resp.Status,
		"headers":    e.headersToMap(resp.Header),
	}

	// Try to parse as JSON
	var jsonBody interface{}
	if err := json.Unmarshal(bodyBytes, &jsonBody); err == nil {
		result["body"] = jsonBody
		result["bodyType"] = "json"
	} else {
		// Return as string if not JSON
		result["body"] = string(bodyBytes)
		result["bodyType"] = "text"
	}

	// Add success flag
	result["success"] = resp.StatusCode >= 200 && resp.StatusCode < 300

	return result, nil
}

// headersToMap converts http.Header to map[string]string
func (e *HTTPNodeExecutor) headersToMap(headers http.Header) map[string]string {
	result := make(map[string]string)
	for key, values := range headers {
		if len(values) > 0 {
			result[key] = values[0]
		}
	}
	return result
}

// interpolateVariables replaces {{variable}} with actual values
func (e *HTTPNodeExecutor) interpolateVariables(template string, variables map[string]interface{}) string {
	result := template
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result
}

// interpolateInObject recursively interpolates variables in an object
func (e *HTTPNodeExecutor) interpolateInObject(obj interface{}, variables map[string]interface{}) interface{} {
	switch v := obj.(type) {
	case string:
		return e.interpolateVariables(v, variables)
	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			result[key] = e.interpolateInObject(value, variables)
		}
		return result
	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			result[i] = e.interpolateInObject(item, variables)
		}
		return result
	default:
		return obj
	}
}
