package types

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// ConditionalNodeExecutor handles IF logic
type ConditionalNodeExecutor struct {
	BaseNodeExecutor
}

func NewConditionalNodeExecutor() *ConditionalNodeExecutor {
	return &ConditionalNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *ConditionalNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters

	conditions, ok := params["conditions"].([]interface{})
	if !ok {
		// Single condition mode
		condition, _ := params["condition"].(map[string]interface{})
		result, err := evaluateCondition(input, condition)
		if err != nil {
			return nil, fmt.Errorf("failed to evaluate condition: %w", err)
		}

		return map[string]interface{}{
			"result": result,
			"branch": getBranch(result),
		}, nil
	}

	// Multiple conditions (AND/OR)
	combineMode, _ := params["combineMode"].(string)
	if combineMode == "" {
		combineMode = "and"
	}

	results := make([]bool, len(conditions))
	for i, cond := range conditions {
		condMap, ok := cond.(map[string]interface{})
		if !ok {
			continue
		}
		result, err := evaluateCondition(input, condMap)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}

	var finalResult bool
	if combineMode == "and" {
		finalResult = true
		for _, r := range results {
			if !r {
				finalResult = false
				break
			}
		}
	} else {
		finalResult = false
		for _, r := range results {
			if r {
				finalResult = true
				break
			}
		}
	}

	return map[string]interface{}{
		"result":     finalResult,
		"branch":     getBranch(finalResult),
		"conditions": results,
	}, nil
}

func getBranch(result bool) string {
	if result {
		return "true"
	}
	return "false"
}

func evaluateCondition(input map[string]interface{}, condition map[string]interface{}) (bool, error) {
	field, _ := condition["field"].(string)
	operator, _ := condition["operator"].(string)
	value := condition["value"]

	fieldValue := getNestedValue(input, field)

	switch operator {
	case "equals", "==", "eq":
		return compareEquals(fieldValue, value), nil
	case "notEquals", "!=", "ne":
		return !compareEquals(fieldValue, value), nil
	case "contains":
		return compareContains(fieldValue, value), nil
	case "notContains":
		return !compareContains(fieldValue, value), nil
	case "startsWith":
		return compareStartsWith(fieldValue, value), nil
	case "endsWith":
		return compareEndsWith(fieldValue, value), nil
	case "greaterThan", ">", "gt":
		return compareGreaterThan(fieldValue, value), nil
	case "lessThan", "<", "lt":
		return compareLessThan(fieldValue, value), nil
	case "greaterThanOrEqual", ">=", "gte":
		return compareGreaterThanOrEqual(fieldValue, value), nil
	case "lessThanOrEqual", "<=", "lte":
		return compareLessThanOrEqual(fieldValue, value), nil
	case "isEmpty":
		return isEmpty(fieldValue), nil
	case "isNotEmpty":
		return !isEmpty(fieldValue), nil
	case "isNull":
		return fieldValue == nil, nil
	case "isNotNull":
		return fieldValue != nil, nil
	case "regex", "matches":
		return compareRegex(fieldValue, value), nil
	case "in":
		return compareIn(fieldValue, value), nil
	case "notIn":
		return !compareIn(fieldValue, value), nil
	case "isTrue":
		return fieldValue == true, nil
	case "isFalse":
		return fieldValue == false, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}

func getNestedValue(data map[string]interface{}, path string) interface{} {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		if current == nil {
			return nil
		}

		// Handle array index
		if idx := strings.Index(part, "["); idx != -1 {
			key := part[:idx]
			indexStr := part[idx+1 : len(part)-1]
			index, _ := strconv.Atoi(indexStr)

			if m, ok := current.(map[string]interface{}); ok {
				current = m[key]
			}

			if arr, ok := current.([]interface{}); ok && index < len(arr) {
				current = arr[index]
			} else {
				return nil
			}
		} else {
			if m, ok := current.(map[string]interface{}); ok {
				current = m[part]
			} else {
				return nil
			}
		}
	}

	return current
}

func compareEquals(a, b interface{}) bool {
	return fmt.Sprintf("%v", a) == fmt.Sprintf("%v", b)
}

func compareContains(a, b interface{}) bool {
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.Contains(aStr, bStr)
}

func compareStartsWith(a, b interface{}) bool {
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.HasPrefix(aStr, bStr)
}

func compareEndsWith(a, b interface{}) bool {
	aStr := fmt.Sprintf("%v", a)
	bStr := fmt.Sprintf("%v", b)
	return strings.HasSuffix(aStr, bStr)
}

func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case string:
		return strconv.ParseFloat(val, 64)
	default:
		return 0, errors.New("cannot convert to float64")
	}
}

func compareGreaterThan(a, b interface{}) bool {
	aNum, err1 := toFloat64(a)
	bNum, err2 := toFloat64(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return aNum > bNum
}

func compareLessThan(a, b interface{}) bool {
	aNum, err1 := toFloat64(a)
	bNum, err2 := toFloat64(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return aNum < bNum
}

func compareGreaterThanOrEqual(a, b interface{}) bool {
	aNum, err1 := toFloat64(a)
	bNum, err2 := toFloat64(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return aNum >= bNum
}

func compareLessThanOrEqual(a, b interface{}) bool {
	aNum, err1 := toFloat64(a)
	bNum, err2 := toFloat64(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return aNum <= bNum
}

func isEmpty(v interface{}) bool {
	if v == nil {
		return true
	}

	val := reflect.ValueOf(v)
	switch val.Kind() {
	case reflect.String:
		return val.Len() == 0
	case reflect.Array, reflect.Slice, reflect.Map:
		return val.Len() == 0
	default:
		return false
	}
}

func compareRegex(a, b interface{}) bool {
	aStr := fmt.Sprintf("%v", a)
	pattern := fmt.Sprintf("%v", b)

	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}

	return re.MatchString(aStr)
}

func compareIn(a, b interface{}) bool {
	var list []interface{}

	switch v := b.(type) {
	case []interface{}:
		list = v
	case string:
		if err := json.Unmarshal([]byte(v), &list); err != nil {
			parts := strings.Split(v, ",")
			for _, p := range parts {
				list = append(list, strings.TrimSpace(p))
			}
		}
	default:
		return false
	}

	aStr := fmt.Sprintf("%v", a)
	for _, item := range list {
		if fmt.Sprintf("%v", item) == aStr {
			return true
		}
	}

	return false
}

// SwitchNodeExecutor handles multiple branch routing
type SwitchNodeExecutor struct {
	BaseNodeExecutor
}

func NewSwitchNodeExecutor() *SwitchNodeExecutor {
	return &SwitchNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *SwitchNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	field, _ := params["field"].(string)
	cases, _ := params["cases"].([]interface{})
	defaultBranch, _ := params["default"].(string)

	if defaultBranch == "" {
		defaultBranch = "default"
	}

	fieldValue := getNestedValue(input, field)
	fieldStr := fmt.Sprintf("%v", fieldValue)

	for _, c := range cases {
		caseMap, ok := c.(map[string]interface{})
		if !ok {
			continue
		}

		caseValue := fmt.Sprintf("%v", caseMap["value"])
		caseBranch, _ := caseMap["branch"].(string)

		if fieldStr == caseValue {
			return map[string]interface{}{
				"matched": true,
				"value":   fieldValue,
				"case":    caseValue,
				"branch":  caseBranch,
			}, nil
		}
	}

	return map[string]interface{}{
		"matched": false,
		"value":   fieldValue,
		"branch":  defaultBranch,
	}, nil
}
