package types

import (
	"context"
	"fmt"
	"time"
)

// LoopNodeExecutor handles basic loop iteration
type LoopNodeExecutor struct {
	BaseNodeExecutor
}

func NewLoopNodeExecutor() *LoopNodeExecutor {
	return &LoopNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 60 * time.Second},
	}
}

func (e *LoopNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	iterations, _ := params["iterations"].(float64)
	if iterations == 0 {
		iterations = 10
	}

	results := make([]interface{}, int(iterations))
	for i := 0; i < int(iterations); i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			results[i] = map[string]interface{}{
				"index": i,
				"input": input,
			}
		}
	}

	return map[string]interface{}{
		"items":      results,
		"totalCount": int(iterations),
	}, nil
}

// ForEachNodeExecutor iterates over arrays
type ForEachNodeExecutor struct {
	BaseNodeExecutor
}

func NewForEachNodeExecutor() *ForEachNodeExecutor {
	return &ForEachNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 60 * time.Second},
	}
}

func (e *ForEachNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	field, _ := params["field"].(string)
	batchSize, _ := params["batchSize"].(float64)

	items := getNestedValue(input, field)
	itemsSlice, ok := items.([]interface{})
	if !ok {
		return nil, fmt.Errorf("field %s is not an array", field)
	}

	results := make([]interface{}, len(itemsSlice))
	for i, item := range itemsSlice {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			results[i] = map[string]interface{}{
				"index": i,
				"item":  item,
			}
		}
	}

	output := map[string]interface{}{
		"items":      results,
		"totalCount": len(itemsSlice),
	}

	if batchSize > 0 {
		batches := make([][]interface{}, 0)
		for i := 0; i < len(results); i += int(batchSize) {
			end := i + int(batchSize)
			if end > len(results) {
				end = len(results)
			}
			batches = append(batches, results[i:end])
		}
		output["batches"] = batches
		output["batchCount"] = len(batches)
	}

	return output, nil
}

// WhileNodeExecutor executes while condition is true
type WhileNodeExecutor struct {
	BaseNodeExecutor
}

func NewWhileNodeExecutor() *WhileNodeExecutor {
	return &WhileNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 60 * time.Second},
	}
}

func (e *WhileNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	condition, _ := params["condition"].(map[string]interface{})
	maxIterations, _ := params["maxIterations"].(float64)
	if maxIterations == 0 {
		maxIterations = 100
	}

	results := make([]interface{}, 0)
	iteration := 0

	for iteration < int(maxIterations) {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result, err := evaluateCondition(input, condition)
		if err != nil {
			return nil, err
		}

		if !result {
			break
		}

		results = append(results, map[string]interface{}{
			"iteration": iteration,
			"input":     input,
		})
		iteration++
	}

	return map[string]interface{}{
		"items":      results,
		"iterations": iteration,
		"completed":  iteration < int(maxIterations),
	}, nil
}

// SplitNodeExecutor splits data into multiple branches
type SplitNodeExecutor struct {
	BaseNodeExecutor
}

func NewSplitNodeExecutor() *SplitNodeExecutor {
	return &SplitNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *SplitNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	field, _ := params["field"].(string)

	items := getNestedValue(input, field)
	itemsSlice, ok := items.([]interface{})
	if !ok {
		return map[string]interface{}{
			"branches": []interface{}{input},
			"count":    1,
		}, nil
	}

	branches := make([]interface{}, len(itemsSlice))
	for i, item := range itemsSlice {
		branches[i] = map[string]interface{}{
			"index": i,
			"data":  item,
		}
	}

	return map[string]interface{}{
		"branches": branches,
		"count":    len(branches),
	}, nil
}

// MergeNodeExecutor merges multiple inputs
type MergeNodeExecutor struct {
	BaseNodeExecutor
}

func NewMergeNodeExecutor() *MergeNodeExecutor {
	return &MergeNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *MergeNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	mode, _ := params["mode"].(string)

	if mode == "" {
		mode = "append"
	}

	inputs, _ := input["inputs"].([]interface{})
	if inputs == nil {
		inputs = []interface{}{input}
	}

	switch mode {
	case "append":
		merged := make([]interface{}, 0)
		for _, inp := range inputs {
			if arr, ok := inp.([]interface{}); ok {
				merged = append(merged, arr...)
			} else {
				merged = append(merged, inp)
			}
		}
		return map[string]interface{}{
			"merged": merged,
			"count":  len(merged),
		}, nil

	case "combine":
		combined := make(map[string]interface{})
		for _, inp := range inputs {
			if m, ok := inp.(map[string]interface{}); ok {
				for k, v := range m {
					combined[k] = v
				}
			}
		}
		return map[string]interface{}{
			"merged": combined,
		}, nil

	case "wait":
		return map[string]interface{}{
			"inputs": inputs,
			"count":  len(inputs),
		}, nil

	default:
		return map[string]interface{}{
			"inputs": inputs,
		}, nil
	}
}

// AggregateNodeExecutor aggregates data
type AggregateNodeExecutor struct {
	BaseNodeExecutor
}

func NewAggregateNodeExecutor() *AggregateNodeExecutor {
	return &AggregateNodeExecutor{
		BaseNodeExecutor: BaseNodeExecutor{timeout: 30 * time.Second},
	}
}

func (e *AggregateNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	params := node.Parameters
	field, _ := params["field"].(string)
	operation, _ := params["operation"].(string)

	items := getNestedValue(input, field)
	itemsSlice, ok := items.([]interface{})
	if !ok {
		return nil, fmt.Errorf("field %s is not an array", field)
	}

	result := map[string]interface{}{
		"count": len(itemsSlice),
	}

	switch operation {
	case "sum":
		var sum float64
		for _, item := range itemsSlice {
			if num, err := toFloat64(item); err == nil {
				sum += num
			}
		}
		result["sum"] = sum

	case "avg", "average":
		var sum float64
		count := 0
		for _, item := range itemsSlice {
			if num, err := toFloat64(item); err == nil {
				sum += num
				count++
			}
		}
		if count > 0 {
			result["average"] = sum / float64(count)
		}

	case "min":
		var min float64
		first := true
		for _, item := range itemsSlice {
			if num, err := toFloat64(item); err == nil {
				if first || num < min {
					min = num
					first = false
				}
			}
		}
		if !first {
			result["min"] = min
		}

	case "max":
		var max float64
		first := true
		for _, item := range itemsSlice {
			if num, err := toFloat64(item); err == nil {
				if first || num > max {
					max = num
					first = false
				}
			}
		}
		if !first {
			result["max"] = max
		}

	case "concat":
		var concat string
		separator, _ := params["separator"].(string)
		for i, item := range itemsSlice {
			if i > 0 && separator != "" {
				concat += separator
			}
			concat += fmt.Sprintf("%v", item)
		}
		result["concat"] = concat

	case "unique":
		seen := make(map[string]bool)
		unique := make([]interface{}, 0)
		for _, item := range itemsSlice {
			key := fmt.Sprintf("%v", item)
			if !seen[key] {
				seen[key] = true
				unique = append(unique, item)
			}
		}
		result["unique"] = unique
		result["uniqueCount"] = len(unique)

	case "group":
		groupBy, _ := params["groupBy"].(string)
		groups := make(map[string][]interface{})
		for _, item := range itemsSlice {
			if m, ok := item.(map[string]interface{}); ok {
				key := fmt.Sprintf("%v", m[groupBy])
				groups[key] = append(groups[key], item)
			}
		}
		result["groups"] = groups
		result["groupCount"] = len(groups)

	default:
		result["items"] = itemsSlice
	}

	return result, nil
}
