package types

import (
	"context"
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash"
	"math"
	"regexp"
	"strings"
	"time"
)

// SetNodeExecutor sets values in the data
type SetNodeExecutor struct {
	BaseNodeExecutor
}

func NewSetNodeExecutor() *SetNodeExecutor {
	return &SetNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *SetNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	values, _ := params["values"].(map[string]interface{})
	keepExisting, _ := params["keepExisting"].(bool)

	result := make(map[string]interface{})
	if keepExisting {
		for k, v := range input {
			result[k] = v
		}
	}

	for k, v := range values {
		result[k] = v
	}

	return result, nil
}

// FunctionNodeExecutor executes custom expressions
type FunctionNodeExecutor struct {
	BaseNodeExecutor
}

func NewFunctionNodeExecutor() *FunctionNodeExecutor {
	return &FunctionNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *FunctionNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	expression, _ := params["expression"].(string)
	returnField, _ := params["returnField"].(string)

	if returnField == "" {
		returnField = "result"
	}

	// Simple expression evaluation (in production, use a proper expression engine)
	result := evaluateExpression(expression, input)

	return map[string]interface{}{
		returnField: result,
		"input":     input,
	}, nil
}

func evaluateExpression(expr string, data map[string]interface{}) interface{} {
	// Simple variable substitution
	result := expr
	for k, v := range data {
		placeholder := fmt.Sprintf("{{%s}}", k)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", v))
	}
	return result
}

// WaitNodeExecutor pauses execution
type WaitNodeExecutor struct {
	BaseNodeExecutor
}

func NewWaitNodeExecutor() *WaitNodeExecutor {
	return &WaitNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 300 * time.Second},
	}
}

func (e *WaitNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	duration, _ := params["duration"].(float64)
	unit, _ := params["unit"].(string)

	if unit == "" {
		unit = "seconds"
	}

	var waitDuration time.Duration
	switch unit {
	case "milliseconds", "ms":
		waitDuration = time.Duration(duration) * time.Millisecond
	case "seconds", "s":
		waitDuration = time.Duration(duration) * time.Second
	case "minutes", "m":
		waitDuration = time.Duration(duration) * time.Minute
	case "hours", "h":
		waitDuration = time.Duration(duration) * time.Hour
	default:
		waitDuration = time.Duration(duration) * time.Second
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-time.After(waitDuration):
	}

	return map[string]interface{}{
		"waited":   true,
		"duration": waitDuration.String(),
		"input":    input,
	}, nil
}

// DateTimeNodeExecutor handles date/time operations
type DateTimeNodeExecutor struct {
	BaseNodeExecutor
}

func NewDateTimeNodeExecutor() *DateTimeNodeExecutor {
	return &DateTimeNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *DateTimeNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	operation, _ := params["operation"].(string)
	timezone, _ := params["timezone"].(string)

	loc := time.UTC
	if timezone != "" {
		if l, err := time.LoadLocation(timezone); err == nil {
			loc = l
		}
	}

	now := time.Now().In(loc)

	switch operation {
	case "now":
		return map[string]interface{}{
			"iso":       now.Format(time.RFC3339),
			"unix":      now.Unix(),
			"unixMilli": now.UnixMilli(),
			"year":      now.Year(),
			"month":     int(now.Month()),
			"day":       now.Day(),
			"hour":      now.Hour(),
			"minute":    now.Minute(),
			"second":    now.Second(),
			"weekday":   now.Weekday().String(),
		}, nil

	case "format":
		format, _ := params["format"].(string)
		dateStr, _ := params["date"].(string)
		var t time.Time
		if dateStr != "" {
			t, _ = time.Parse(time.RFC3339, dateStr)
		} else {
			t = now
		}
		return map[string]interface{}{
			"formatted": t.Format(format),
		}, nil

	case "parse":
		dateStr, _ := params["date"].(string)
		format, _ := params["format"].(string)
		t, err := time.Parse(format, dateStr)
		if err != nil {
			return nil, fmt.Errorf("failed to parse date: %w", err)
		}
		return map[string]interface{}{
			"iso":  t.Format(time.RFC3339),
			"unix": t.Unix(),
		}, nil

	case "add":
		amount, _ := params["amount"].(float64)
		unit, _ := params["unit"].(string)
		var d time.Duration
		switch unit {
		case "seconds":
			d = time.Duration(amount) * time.Second
		case "minutes":
			d = time.Duration(amount) * time.Minute
		case "hours":
			d = time.Duration(amount) * time.Hour
		case "days":
			d = time.Duration(amount) * 24 * time.Hour
		}
		result := now.Add(d)
		return map[string]interface{}{
			"iso":  result.Format(time.RFC3339),
			"unix": result.Unix(),
		}, nil

	case "diff":
		date1, _ := params["date1"].(string)
		date2, _ := params["date2"].(string)
		t1, _ := time.Parse(time.RFC3339, date1)
		t2, _ := time.Parse(time.RFC3339, date2)
		diff := t2.Sub(t1)
		return map[string]interface{}{
			"seconds": diff.Seconds(),
			"minutes": diff.Minutes(),
			"hours":   diff.Hours(),
			"days":    diff.Hours() / 24,
		}, nil

	default:
		return map[string]interface{}{
			"iso":  now.Format(time.RFC3339),
			"unix": now.Unix(),
		}, nil
	}
}

// CryptoNodeExecutor handles cryptographic operations
type CryptoNodeExecutor struct {
	BaseNodeExecutor
}

func NewCryptoNodeExecutor() *CryptoNodeExecutor {
	return &CryptoNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *CryptoNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	operation, _ := params["operation"].(string)
	data, _ := params["data"].(string)

	switch operation {
	case "hash":
		algorithm, _ := params["algorithm"].(string)
		var h hash.Hash
		switch algorithm {
		case "md5":
			h = md5.New()
		case "sha1":
			h = sha1.New()
		case "sha256":
			h = sha256.New()
		case "sha512":
			h = sha512.New()
		default:
			h = sha256.New()
		}
		h.Write([]byte(data))
		return map[string]interface{}{
			"hash":      hex.EncodeToString(h.Sum(nil)),
			"algorithm": algorithm,
		}, nil

	case "hmac":
		algorithm, _ := params["algorithm"].(string)
		secret, _ := params["secret"].(string)
		var h func() hash.Hash
		switch algorithm {
		case "sha1":
			h = sha1.New
		case "sha256":
			h = sha256.New
		case "sha512":
			h = sha512.New
		default:
			h = sha256.New
		}
		mac := hmac.New(h, []byte(secret))
		mac.Write([]byte(data))
		return map[string]interface{}{
			"hmac":      hex.EncodeToString(mac.Sum(nil)),
			"algorithm": algorithm,
		}, nil

	case "base64Encode":
		return map[string]interface{}{
			"encoded": base64.StdEncoding.EncodeToString([]byte(data)),
		}, nil

	case "base64Decode":
		decoded, err := base64.StdEncoding.DecodeString(data)
		if err != nil {
			return nil, fmt.Errorf("failed to decode base64: %w", err)
		}
		return map[string]interface{}{
			"decoded": string(decoded),
		}, nil

	default:
		return nil, fmt.Errorf("unknown crypto operation: %s", operation)
	}
}

// JSONNodeExecutor handles JSON operations
type JSONNodeExecutor struct {
	BaseNodeExecutor
}

func NewJSONNodeExecutor() *JSONNodeExecutor {
	return &JSONNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *JSONNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	operation, _ := params["operation"].(string)

	switch operation {
	case "parse":
		jsonStr, _ := params["json"].(string)
		var result interface{}
		if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
			return nil, fmt.Errorf("failed to parse JSON: %w", err)
		}
		return map[string]interface{}{
			"parsed": result,
		}, nil

	case "stringify":
		data := params["data"]
		pretty, _ := params["pretty"].(bool)
		var jsonBytes []byte
		var err error
		if pretty {
			jsonBytes, err = json.MarshalIndent(data, "", "  ")
		} else {
			jsonBytes, err = json.Marshal(data)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to stringify: %w", err)
		}
		return map[string]interface{}{
			"json": string(jsonBytes),
		}, nil

	case "get":
		path, _ := params["path"].(string)
		value := getNestedValue(input, path)
		return map[string]interface{}{
			"value": value,
		}, nil

	case "set":
		path, _ := params["path"].(string)
		value := params["value"]
		result := setNestedValue(input, path, value)
		return result, nil

	default:
		return input, nil
	}
}

func setNestedValue(data map[string]interface{}, path string, value interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range data {
		result[k] = v
	}

	parts := strings.Split(path, ".")
	current := result
	for i, part := range parts {
		if i == len(parts)-1 {
			current[part] = value
		} else {
			if _, ok := current[part]; !ok {
				current[part] = make(map[string]interface{})
			}
			if m, ok := current[part].(map[string]interface{}); ok {
				current = m
			}
		}
	}

	return result
}

// MathNodeExecutor handles mathematical operations
type MathNodeExecutor struct {
	BaseNodeExecutor
}

func NewMathNodeExecutor() *MathNodeExecutor {
	return &MathNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *MathNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	operation, _ := params["operation"].(string)
	a, _ := toFloat64(params["a"])
	b, _ := toFloat64(params["b"])

	var result float64
	switch operation {
	case "add", "+":
		result = a + b
	case "subtract", "-":
		result = a - b
	case "multiply", "*":
		result = a * b
	case "divide", "/":
		if b == 0 {
			return nil, fmt.Errorf("division by zero")
		}
		result = a / b
	case "modulo", "%":
		result = math.Mod(a, b)
	case "power", "^":
		result = math.Pow(a, b)
	case "sqrt":
		result = math.Sqrt(a)
	case "abs":
		result = math.Abs(a)
	case "ceil":
		result = math.Ceil(a)
	case "floor":
		result = math.Floor(a)
	case "round":
		result = math.Round(a)
	case "min":
		result = math.Min(a, b)
	case "max":
		result = math.Max(a, b)
	case "random":
		result = math.Floor(a + (b-a+1)*float64(time.Now().UnixNano()%1000)/1000)
	default:
		return nil, fmt.Errorf("unknown math operation: %s", operation)
	}

	return map[string]interface{}{
		"result": result,
	}, nil
}

// TextNodeExecutor handles text operations
type TextNodeExecutor struct {
	BaseNodeExecutor
}

func NewTextNodeExecutor() *TextNodeExecutor {
	return &TextNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *TextNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	operation, _ := params["operation"].(string)
	text, _ := params["text"].(string)

	switch operation {
	case "uppercase":
		return map[string]interface{}{"result": strings.ToUpper(text)}, nil
	case "lowercase":
		return map[string]interface{}{"result": strings.ToLower(text)}, nil
	case "trim":
		return map[string]interface{}{"result": strings.TrimSpace(text)}, nil
	case "split":
		separator, _ := params["separator"].(string)
		return map[string]interface{}{"result": strings.Split(text, separator)}, nil
	case "join":
		items, _ := params["items"].([]interface{})
		separator, _ := params["separator"].(string)
		strs := make([]string, len(items))
		for i, item := range items {
			strs[i] = fmt.Sprintf("%v", item)
		}
		return map[string]interface{}{"result": strings.Join(strs, separator)}, nil
	case "replace":
		old, _ := params["old"].(string)
		new, _ := params["new"].(string)
		return map[string]interface{}{"result": strings.ReplaceAll(text, old, new)}, nil
	case "substring":
		start, _ := params["start"].(float64)
		length, _ := params["length"].(float64)
		end := int(start + length)
		if end > len(text) {
			end = len(text)
		}
		return map[string]interface{}{"result": text[int(start):end]}, nil
	case "length":
		return map[string]interface{}{"result": len(text)}, nil
	case "contains":
		search, _ := params["search"].(string)
		return map[string]interface{}{"result": strings.Contains(text, search)}, nil
	case "regex":
		pattern, _ := params["pattern"].(string)
		re, err := regexp.Compile(pattern)
		if err != nil {
			return nil, fmt.Errorf("invalid regex: %w", err)
		}
		matches := re.FindAllString(text, -1)
		return map[string]interface{}{"matches": matches, "count": len(matches)}, nil
	default:
		return map[string]interface{}{"result": text}, nil
	}
}
