package workflow

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Variable types
const (
	VarTypeString  = "string"
	VarTypeNumber  = "number"
	VarTypeBoolean = "boolean"
	VarTypeJSON    = "json"
	VarTypeSecret  = "secret"
	VarTypeArray   = "array"
	VarTypeObject  = "object"
)

// Variable scopes
const (
	ScopeGlobal    = "global"
	ScopeWorkflow  = "workflow"
	ScopeExecution = "execution"
	ScopeNode      = "node"
)

var (
	ErrVariableNotFound    = errors.New("variable not found")
	ErrInvalidVariableType = errors.New("invalid variable type")
	ErrVariableReadOnly    = errors.New("variable is read-only")
	ErrCircularReference   = errors.New("circular variable reference detected")
	ErrInvalidVariableName = errors.New("invalid variable name")
)

// Variable represents a workflow variable
type WorkflowVariable struct {
	Key         string      `json:"key" gorm:"primaryKey"`
	WorkflowID  string      `json:"workflowId" gorm:"primaryKey;index"`
	Name        string      `json:"name"`
	Type        string      `json:"type"`
	Value       interface{} `json:"value" gorm:"serializer:json"`
	Description string      `json:"description"`
	Scope       string      `json:"scope"`
	Environment string      `json:"environment"`
	Encrypted   bool        `json:"encrypted"`
	ReadOnly    bool        `json:"readOnly"`
	Required    bool        `json:"required"`
	CreatedAt   string      `json:"createdAt"`
	UpdatedAt   string      `json:"updatedAt"`
}

// Environment represents an execution environment
type Environment struct {
	ID          string                 `json:"id" gorm:"primaryKey"`
	WorkflowID  string                 `json:"workflowId" gorm:"index"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	Variables   map[string]interface{} `json:"variables" gorm:"serializer:json"`
	IsDefault   bool                   `json:"isDefault"`
	CreatedAt   string                 `json:"createdAt"`
	UpdatedAt   string                 `json:"updatedAt"`
}

// VariableContext manages variables during workflow execution
type VariableContext struct {
	globalVars    map[string]interface{}
	workflowVars  map[string]interface{}
	executionVars map[string]interface{}
	nodeVars      map[string]map[string]interface{}
	environment   *Environment
	readOnly      map[string]bool
	encrypted     map[string]bool
}

// NewVariableContext creates a new variable context
func NewVariableContext() *VariableContext {
	return &VariableContext{
		globalVars:    make(map[string]interface{}),
		workflowVars:  make(map[string]interface{}),
		executionVars: make(map[string]interface{}),
		nodeVars:      make(map[string]map[string]interface{}),
		readOnly:      make(map[string]bool),
		encrypted:     make(map[string]bool),
	}
}

// SetGlobalVariable sets a global variable
func (vc *VariableContext) SetGlobalVariable(key string, value interface{}) error {
	if vc.readOnly[key] {
		return ErrVariableReadOnly
	}
	vc.globalVars[key] = value
	return nil
}

// SetWorkflowVariable sets a workflow-scoped variable
func (vc *VariableContext) SetWorkflowVariable(key string, value interface{}) error {
	if vc.readOnly[key] {
		return ErrVariableReadOnly
	}
	vc.workflowVars[key] = value
	return nil
}

// SetExecutionVariable sets an execution-scoped variable
func (vc *VariableContext) SetExecutionVariable(key string, value interface{}) error {
	if vc.readOnly[key] {
		return ErrVariableReadOnly
	}
	vc.executionVars[key] = value
	return nil
}

// SetNodeVariable sets a node-scoped variable
func (vc *VariableContext) SetNodeVariable(nodeID, key string, value interface{}) error {
	if vc.readOnly[key] {
		return ErrVariableReadOnly
	}
	if vc.nodeVars[nodeID] == nil {
		vc.nodeVars[nodeID] = make(map[string]interface{})
	}
	vc.nodeVars[nodeID][key] = value
	return nil
}

// GetVariable retrieves a variable value with scope resolution
func (vc *VariableContext) GetVariable(key string, nodeID string) (interface{}, error) {
	// Check node scope first if nodeID is provided
	if nodeID != "" {
		if nodeVars, ok := vc.nodeVars[nodeID]; ok {
			if value, exists := nodeVars[key]; exists {
				return value, nil
			}
		}
	}

	// Check execution scope
	if value, ok := vc.executionVars[key]; ok {
		return value, nil
	}

	// Check workflow scope
	if value, ok := vc.workflowVars[key]; ok {
		return value, nil
	}

	// Check environment variables
	if vc.environment != nil {
		if value, ok := vc.environment.Variables[key]; ok {
			return value, nil
		}
	}

	// Check global scope
	if value, ok := vc.globalVars[key]; ok {
		return value, nil
	}

	// Check system environment variables
	if envValue := os.Getenv(key); envValue != "" {
		return envValue, nil
	}

	return nil, ErrVariableNotFound
}

// SetEnvironment sets the current environment
func (vc *VariableContext) SetEnvironment(env *Environment) {
	vc.environment = env
}

// MarkReadOnly marks a variable as read-only
func (vc *VariableContext) MarkReadOnly(key string) {
	vc.readOnly[key] = true
}

// MarkEncrypted marks a variable as encrypted
func (vc *VariableContext) MarkEncrypted(key string) {
	vc.encrypted[key] = true
}

// IsEncrypted checks if a variable is encrypted
func (vc *VariableContext) IsEncrypted(key string) bool {
	return vc.encrypted[key]
}

// InterpolateString interpolates variables in a string
func (vc *VariableContext) InterpolateString(input string, nodeID string) (string, error) {
	// Regular expression to match {{variable}} or ${variable}
	re := regexp.MustCompile(`\{\{([^}]+)\}\}|\$\{([^}]+)\}`)

	var lastErr error
	result := re.ReplaceAllStringFunc(input, func(match string) string {
		// Extract variable name
		varName := strings.Trim(match, "{}$")
		varName = strings.TrimSpace(varName)

		// Get variable value
		value, err := vc.GetVariable(varName, nodeID)
		if err != nil {
			lastErr = err
			return match // Keep original if not found
		}

		// Convert value to string
		switch v := value.(type) {
		case string:
			return v
		case int, int32, int64, float32, float64:
			return fmt.Sprintf("%v", v)
		case bool:
			return fmt.Sprintf("%v", v)
		default:
			// For complex types, marshal to JSON
			jsonBytes, err := json.Marshal(v)
			if err != nil {
				lastErr = err
				return match
			}
			return string(jsonBytes)
		}
	})

	return result, lastErr
}

// InterpolateObject interpolates variables in an object (map or struct)
func (vc *VariableContext) InterpolateObject(input interface{}, nodeID string) (interface{}, error) {
	switch v := input.(type) {
	case string:
		return vc.InterpolateString(v, nodeID)

	case map[string]interface{}:
		result := make(map[string]interface{})
		for key, value := range v {
			interpolatedValue, err := vc.InterpolateObject(value, nodeID)
			if err != nil {
				return nil, err
			}
			result[key] = interpolatedValue
		}
		return result, nil

	case []interface{}:
		result := make([]interface{}, len(v))
		for i, item := range v {
			interpolatedItem, err := vc.InterpolateObject(item, nodeID)
			if err != nil {
				return nil, err
			}
			result[i] = interpolatedItem
		}
		return result, nil

	default:
		// Return as-is for other types
		return input, nil
	}
}

// Clone creates a copy of the variable context
func (vc *VariableContext) Clone() *VariableContext {
	clone := NewVariableContext()

	// Deep copy maps
	for k, v := range vc.globalVars {
		clone.globalVars[k] = v
	}
	for k, v := range vc.workflowVars {
		clone.workflowVars[k] = v
	}
	for k, v := range vc.executionVars {
		clone.executionVars[k] = v
	}
	for nodeID, vars := range vc.nodeVars {
		clone.nodeVars[nodeID] = make(map[string]interface{})
		for k, v := range vars {
			clone.nodeVars[nodeID][k] = v
		}
	}
	for k, v := range vc.readOnly {
		clone.readOnly[k] = v
	}
	for k, v := range vc.encrypted {
		clone.encrypted[k] = v
	}

	clone.environment = vc.environment

	return clone
}

// ExportVariables exports all variables as a map
func (vc *VariableContext) ExportVariables() map[string]interface{} {
	result := make(map[string]interface{})

	// Add all variables in order of precedence (reverse)
	for k, v := range vc.globalVars {
		result[k] = v
	}

	if vc.environment != nil {
		for k, v := range vc.environment.Variables {
			result[k] = v
		}
	}

	for k, v := range vc.workflowVars {
		result[k] = v
	}

	for k, v := range vc.executionVars {
		result[k] = v
	}

	// Don't export encrypted values
	for k := range result {
		if vc.encrypted[k] {
			result[k] = "***ENCRYPTED***"
		}
	}

	return result
}

// ValidateVariableName validates a variable name
func ValidateVariableName(name string) error {
	if name == "" {
		return ErrInvalidVariableName
	}

	// Variable names must start with letter or underscore
	// and contain only letters, numbers, and underscores
	validNameRegex := regexp.MustCompile(`^[a-zA-Z_][a-zA-Z0-9_]*$`)

	if !validNameRegex.MatchString(name) {
		return fmt.Errorf("%w: must start with letter or underscore and contain only alphanumeric characters and underscores", ErrInvalidVariableName)
	}

	// Check for reserved names
	reservedNames := map[string]bool{
		"true": true, "false": true, "null": true,
		"undefined": true, "this": true, "self": true,
	}

	if reservedNames[strings.ToLower(name)] {
		return fmt.Errorf("%w: '%s' is a reserved name", ErrInvalidVariableName, name)
	}

	return nil
}

// ParseVariableType parses and validates a variable type
func ParseVariableType(value interface{}) string {
	switch value.(type) {
	case string:
		return VarTypeString
	case int, int32, int64, float32, float64:
		return VarTypeNumber
	case bool:
		return VarTypeBoolean
	case []interface{}:
		return VarTypeArray
	case map[string]interface{}:
		return VarTypeObject
	default:
		return VarTypeJSON
	}
}

// CoerceVariableType attempts to coerce a value to the specified type
func CoerceVariableType(value interface{}, targetType string) (interface{}, error) {
	switch targetType {
	case VarTypeString:
		switch v := value.(type) {
		case string:
			return v, nil
		default:
			return fmt.Sprintf("%v", v), nil
		}

	case VarTypeNumber:
		switch v := value.(type) {
		case float64:
			return v, nil
		case int:
			return float64(v), nil
		case string:
			var num float64
			if _, err := fmt.Sscanf(v, "%f", &num); err != nil {
				return nil, fmt.Errorf("cannot convert %s to number", v)
			}
			return num, nil
		default:
			return nil, ErrInvalidVariableType
		}

	case VarTypeBoolean:
		switch v := value.(type) {
		case bool:
			return v, nil
		case string:
			return strings.ToLower(v) == "true", nil
		case int:
			return v != 0, nil
		default:
			return nil, ErrInvalidVariableType
		}

	case VarTypeJSON, VarTypeArray, VarTypeObject:
		// These types are stored as-is
		return value, nil

	default:
		return nil, fmt.Errorf("unknown type: %s", targetType)
	}
}

// VariableManager manages workflow variables
type VariableManager struct {
	variables    map[string]map[string]*WorkflowVariable // workflowID -> key -> variable
	environments map[string][]*Environment               // workflowID -> environments
}

// NewVariableManager creates a new variable manager
func NewVariableManager() *VariableManager {
	return &VariableManager{
		variables:    make(map[string]map[string]*WorkflowVariable),
		environments: make(map[string][]*Environment),
	}
}

// SetVariable sets a workflow variable
func (vm *VariableManager) SetVariable(workflowID string, variable *WorkflowVariable) error {
	if err := ValidateVariableName(variable.Key); err != nil {
		return err
	}

	if vm.variables[workflowID] == nil {
		vm.variables[workflowID] = make(map[string]*WorkflowVariable)
	}

	vm.variables[workflowID][variable.Key] = variable
	return nil
}

// GetVariable retrieves a workflow variable
func (vm *VariableManager) GetVariable(workflowID, key string) (*WorkflowVariable, error) {
	if wfVars, ok := vm.variables[workflowID]; ok {
		if variable, exists := wfVars[key]; exists {
			return variable, nil
		}
	}
	return nil, ErrVariableNotFound
}

// ListVariables lists all variables for a workflow
func (vm *VariableManager) ListVariables(workflowID string) []*WorkflowVariable {
	result := []*WorkflowVariable{}

	if wfVars, ok := vm.variables[workflowID]; ok {
		for _, variable := range wfVars {
			result = append(result, variable)
		}
	}

	return result
}

// DeleteVariable deletes a workflow variable
func (vm *VariableManager) DeleteVariable(workflowID, key string) error {
	if wfVars, ok := vm.variables[workflowID]; ok {
		delete(wfVars, key)
		return nil
	}
	return ErrVariableNotFound
}

// SetEnvironment sets an environment for a workflow
func (vm *VariableManager) SetEnvironment(workflowID string, env *Environment) {
	if vm.environments[workflowID] == nil {
		vm.environments[workflowID] = []*Environment{}
	}

	// Check if environment exists and update, otherwise append
	found := false
	for i, existing := range vm.environments[workflowID] {
		if existing.ID == env.ID {
			vm.environments[workflowID][i] = env
			found = true
			break
		}
	}

	if !found {
		vm.environments[workflowID] = append(vm.environments[workflowID], env)
	}
}

// GetEnvironment retrieves an environment
func (vm *VariableManager) GetEnvironment(workflowID, envID string) (*Environment, error) {
	if envs, ok := vm.environments[workflowID]; ok {
		for _, env := range envs {
			if env.ID == envID {
				return env, nil
			}
		}
	}
	return nil, errors.New("environment not found")
}

// ListEnvironments lists all environments for a workflow
func (vm *VariableManager) ListEnvironments(workflowID string) []*Environment {
	if envs, ok := vm.environments[workflowID]; ok {
		return envs
	}
	return []*Environment{}
}

// GetDefaultEnvironment gets the default environment for a workflow
func (vm *VariableManager) GetDefaultEnvironment(workflowID string) (*Environment, error) {
	if envs, ok := vm.environments[workflowID]; ok {
		for _, env := range envs {
			if env.IsDefault {
				return env, nil
			}
		}
	}
	return nil, errors.New("no default environment found")
}
