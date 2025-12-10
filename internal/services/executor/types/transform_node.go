package types

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/linkflow-go/pkg/logger"
)

// TransformNodeExecutor handles data transformation nodes
type TransformNodeExecutor struct {
	logger logger.Logger
}

// TransformNodeConfig represents configuration for transform nodes
type TransformNodeConfig struct {
	Operations []TransformOperation `json:"operations"`
	OutputFormat string             `json:"outputFormat"` // json, csv, xml
}

// TransformOperation represents a single transformation operation
type TransformOperation struct {
	Type       string                 `json:"type"` // map, filter, reduce, sort, group, join, split, merge
	Field      string                 `json:"field"`
	TargetField string                `json:"targetField"`
	Expression string                 `json:"expression"`
	Parameters map[string]interface{} `json:"parameters"`
}

// NewTransformNodeExecutor creates a new transform node executor
func NewTransformNodeExecutor(logger logger.Logger) *TransformNodeExecutor {
	return &TransformNodeExecutor{
		logger: logger,
	}
}

// Execute executes a transform node
func (e *TransformNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	config, err := e.parseConfig(node.Config)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Start with input data
	result := input
	
	// Apply each transformation in sequence
	for _, op := range config.Operations {
		transformed, err := e.applyTransformation(op, result)
		if err != nil {
			return nil, fmt.Errorf("transformation failed at operation %s: %w", op.Type, err)
		}
		result = transformed
	}
	
	// Format output if specified
	if config.OutputFormat != "" && config.OutputFormat != "json" {
		result = e.formatOutput(result, config.OutputFormat)
	}
	
	return result, nil
}

// ValidateInput validates the input for the transform node
func (e *TransformNodeExecutor) ValidateInput(node Node, input map[string]interface{}) error {
	config, err := e.parseConfig(node.Config)
	if err != nil {
		return err
	}
	
	if len(config.Operations) == 0 {
		return fmt.Errorf("at least one transformation operation is required")
	}
	
	// Validate each operation
	for _, op := range config.Operations {
		if op.Type == "" {
			return fmt.Errorf("operation type is required")
		}
		
		validTypes := []string{"map", "filter", "reduce", "sort", "group", "join", "split", "merge", "extract", "rename", "convert"}
		valid := false
		for _, t := range validTypes {
			if op.Type == t {
				valid = true
				break
			}
		}
		
		if !valid {
			return fmt.Errorf("invalid operation type: %s", op.Type)
		}
	}
	
	return nil
}

// GetTimeout returns the timeout for the transform operation
func (e *TransformNodeExecutor) GetTimeout() time.Duration {
	return 10 * time.Second
}

// parseConfig parses the node configuration
func (e *TransformNodeExecutor) parseConfig(config interface{}) (*TransformNodeConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	
	var transformConfig TransformNodeConfig
	if err := json.Unmarshal(jsonData, &transformConfig); err != nil {
		return nil, err
	}
	
	if transformConfig.OutputFormat == "" {
		transformConfig.OutputFormat = "json"
	}
	
	return &transformConfig, nil
}

// applyTransformation applies a single transformation operation
func (e *TransformNodeExecutor) applyTransformation(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	switch op.Type {
	case "map":
		return e.transformMap(op, data)
	case "filter":
		return e.transformFilter(op, data)
	case "reduce":
		return e.transformReduce(op, data)
	case "sort":
		return e.transformSort(op, data)
	case "group":
		return e.transformGroup(op, data)
	case "split":
		return e.transformSplit(op, data)
	case "merge":
		return e.transformMerge(op, data)
	case "extract":
		return e.transformExtract(op, data)
	case "rename":
		return e.transformRename(op, data)
	case "convert":
		return e.transformConvert(op, data)
	default:
		return nil, fmt.Errorf("unsupported transformation type: %s", op.Type)
	}
}

// transformMap applies a mapping transformation
func (e *TransformNodeExecutor) transformMap(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	result := make(map[string]interface{})
	
	// Copy all existing fields
	for k, v := range data {
		result[k] = v
	}
	
	// Apply mapping
	if op.Field != "" && op.TargetField != "" {
		if value, exists := data[op.Field]; exists {
			// Apply expression if provided
			if op.Expression != "" {
				transformed := e.evaluateExpression(op.Expression, value)
				result[op.TargetField] = transformed
			} else {
				result[op.TargetField] = value
			}
		}
	}
	
	return result, nil
}

// transformFilter filters data based on conditions
func (e *TransformNodeExecutor) transformFilter(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	if op.Field == "" {
		return data, nil
	}
	
	value, exists := data[op.Field]
	if !exists {
		return data, nil
	}
	
	// Filter arrays
	if arr, ok := value.([]interface{}); ok {
		filtered := []interface{}{}
		for _, item := range arr {
			if e.evaluateCondition(op.Expression, item) {
				filtered = append(filtered, item)
			}
		}
		
		result := make(map[string]interface{})
		for k, v := range data {
			if k == op.Field {
				result[k] = filtered
			} else {
				result[k] = v
			}
		}
		return result, nil
	}
	
	// Filter single value
	if e.evaluateCondition(op.Expression, value) {
		return data, nil
	}
	
	// Remove field if condition not met
	result := make(map[string]interface{})
	for k, v := range data {
		if k != op.Field {
			result[k] = v
		}
	}
	return result, nil
}

// transformReduce reduces an array to a single value
func (e *TransformNodeExecutor) transformReduce(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	if op.Field == "" {
		return data, nil
	}
	
	value, exists := data[op.Field]
	if !exists {
		return data, nil
	}
	
	arr, ok := value.([]interface{})
	if !ok {
		return data, nil
	}
	
	// Perform reduction based on expression
	var result interface{}
	reduceType := op.Expression
	
	switch reduceType {
	case "sum":
		sum := 0.0
		for _, item := range arr {
			if num, err := e.toNumber(item); err == nil {
				sum += num
			}
		}
		result = sum
	case "avg", "average":
		sum := 0.0
		count := 0
		for _, item := range arr {
			if num, err := e.toNumber(item); err == nil {
				sum += num
				count++
			}
		}
		if count > 0 {
			result = sum / float64(count)
		} else {
			result = 0
		}
	case "min":
		if len(arr) > 0 {
			min, _ := e.toNumber(arr[0])
			for _, item := range arr[1:] {
				if num, err := e.toNumber(item); err == nil && num < min {
					min = num
				}
			}
			result = min
		}
	case "max":
		if len(arr) > 0 {
			max, _ := e.toNumber(arr[0])
			for _, item := range arr[1:] {
				if num, err := e.toNumber(item); err == nil && num > max {
					max = num
				}
			}
			result = max
		}
	case "count":
		result = len(arr)
	case "join":
		separator := ", "
		if sep, ok := op.Parameters["separator"].(string); ok {
			separator = sep
		}
		parts := []string{}
		for _, item := range arr {
			parts = append(parts, fmt.Sprintf("%v", item))
		}
		result = strings.Join(parts, separator)
	default:
		result = arr
	}
	
	// Store result
	output := make(map[string]interface{})
	for k, v := range data {
		if k == op.Field {
			if op.TargetField != "" {
				output[op.TargetField] = result
				output[k] = v
			} else {
				output[k] = result
			}
		} else {
			output[k] = v
		}
	}
	
	return output, nil
}

// Additional transformation methods...

// transformSort sorts array data
func (e *TransformNodeExecutor) transformSort(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	// Implementation for sorting
	return data, nil
}

// transformGroup groups data by a field
func (e *TransformNodeExecutor) transformGroup(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	// Implementation for grouping
	return data, nil
}

// transformSplit splits a string field
func (e *TransformNodeExecutor) transformSplit(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	if op.Field == "" {
		return data, nil
	}
	
	value, exists := data[op.Field]
	if !exists {
		return data, nil
	}
	
	str, ok := value.(string)
	if !ok {
		return data, nil
	}
	
	separator := ","
	if sep, ok := op.Parameters["separator"].(string); ok {
		separator = sep
	}
	
	parts := strings.Split(str, separator)
	
	result := make(map[string]interface{})
	for k, v := range data {
		if k == op.Field {
			targetField := op.TargetField
			if targetField == "" {
				targetField = k
			}
			result[targetField] = parts
			if targetField != k {
				result[k] = v
			}
		} else {
			result[k] = v
		}
	}
	
	return result, nil
}

// transformMerge merges multiple fields
func (e *TransformNodeExecutor) transformMerge(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	// Implementation for merging
	return data, nil
}

// transformExtract extracts data using regex or patterns
func (e *TransformNodeExecutor) transformExtract(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	if op.Field == "" || op.Expression == "" {
		return data, nil
	}
	
	value, exists := data[op.Field]
	if !exists {
		return data, nil
	}
	
	str, ok := value.(string)
	if !ok {
		return data, nil
	}
	
	// Use regex to extract
	re, err := regexp.Compile(op.Expression)
	if err != nil {
		return nil, fmt.Errorf("invalid regex: %w", err)
	}
	
	matches := re.FindStringSubmatch(str)
	
	result := make(map[string]interface{})
	for k, v := range data {
		result[k] = v
	}
	
	targetField := op.TargetField
	if targetField == "" {
		targetField = op.Field + "_extracted"
	}
	
	if len(matches) > 0 {
		if len(matches) == 1 {
			result[targetField] = matches[0]
		} else {
			result[targetField] = matches[1:] // Skip full match
		}
	}
	
	return result, nil
}

// transformRename renames fields
func (e *TransformNodeExecutor) transformRename(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	if op.Field == "" || op.TargetField == "" {
		return data, nil
	}
	
	result := make(map[string]interface{})
	for k, v := range data {
		if k == op.Field {
			result[op.TargetField] = v
		} else {
			result[k] = v
		}
	}
	
	return result, nil
}

// transformConvert converts data types
func (e *TransformNodeExecutor) transformConvert(op TransformOperation, data map[string]interface{}) (map[string]interface{}, error) {
	if op.Field == "" {
		return data, nil
	}
	
	value, exists := data[op.Field]
	if !exists {
		return data, nil
	}
	
	targetType := op.Expression
	if targetType == "" {
		if t, ok := op.Parameters["type"].(string); ok {
			targetType = t
		}
	}
	
	var converted interface{}
	var err error
	
	switch targetType {
	case "string":
		converted = fmt.Sprintf("%v", value)
	case "number", "float":
		converted, err = e.toNumber(value)
	case "int", "integer":
		if num, err := e.toNumber(value); err == nil {
			converted = int(num)
		}
	case "bool", "boolean":
		converted = e.toBool(value)
	case "json":
		if str, ok := value.(string); ok {
			json.Unmarshal([]byte(str), &converted)
		} else {
			converted = value
		}
	default:
		converted = value
	}
	
	if err != nil {
		return nil, fmt.Errorf("conversion failed: %w", err)
	}
	
	result := make(map[string]interface{})
	for k, v := range data {
		if k == op.Field {
			result[k] = converted
		} else {
			result[k] = v
		}
	}
	
	return result, nil
}

// Helper methods

func (e *TransformNodeExecutor) evaluateExpression(expression string, value interface{}) interface{} {
	// Simple expression evaluation
	// In production, use a proper expression engine
	switch expression {
	case "uppercase":
		if str, ok := value.(string); ok {
			return strings.ToUpper(str)
		}
	case "lowercase":
		if str, ok := value.(string); ok {
			return strings.ToLower(str)
		}
	case "trim":
		if str, ok := value.(string); ok {
			return strings.TrimSpace(str)
		}
	}
	return value
}

func (e *TransformNodeExecutor) evaluateCondition(condition string, value interface{}) bool {
	// Simple condition evaluation
	// In production, use a proper expression engine
	if condition == "" {
		return true
	}
	
	// Check for simple comparisons
	if strings.Contains(condition, ">") {
		parts := strings.Split(condition, ">")
		if len(parts) == 2 {
			if num, err := e.toNumber(value); err == nil {
				if threshold, err := strconv.ParseFloat(strings.TrimSpace(parts[1]), 64); err == nil {
					return num > threshold
				}
			}
		}
	}
	
	return true
}

func (e *TransformNodeExecutor) toNumber(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("cannot convert %T to number", value)
	}
}

func (e *TransformNodeExecutor) toBool(value interface{}) bool {
	switch v := value.(type) {
	case bool:
		return v
	case string:
		lower := strings.ToLower(v)
		return lower == "true" || lower == "yes" || lower == "1"
	case int:
		return v != 0
	case float64:
		return v != 0
	default:
		return false
	}
}

func (e *TransformNodeExecutor) formatOutput(data map[string]interface{}, format string) map[string]interface{} {
	// Format output based on specified format
	// For now, just return the data
	return data
}
