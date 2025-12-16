package worker

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/linkflow-go/pkg/logger"
)

// Sandbox provides a secure execution environment for code nodes
type Sandbox struct {
	logger      logger.Logger
	tempDir     string
	maxExecTime time.Duration
	maxMemory   int64
	allowedCmds []string
}

// SandboxConfig contains configuration for the sandbox
type SandboxConfig struct {
	TempDir     string
	MaxExecTime time.Duration
	MaxMemory   int64 // in bytes
	AllowedCmds []string
}

// NewSandbox creates a new sandbox environment
func NewSandbox(config SandboxConfig, logger logger.Logger) (*Sandbox, error) {
	if config.TempDir == "" {
		config.TempDir = "/tmp/linkflow-sandbox"
	}

	if config.MaxExecTime == 0 {
		config.MaxExecTime = 30 * time.Second
	}

	if config.MaxMemory == 0 {
		config.MaxMemory = 256 * 1024 * 1024 // 256MB default
	}

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(config.TempDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create sandbox directory: %w", err)
	}

	return &Sandbox{
		logger:      logger,
		tempDir:     config.TempDir,
		maxExecTime: config.MaxExecTime,
		maxMemory:   config.MaxMemory,
		allowedCmds: config.AllowedCmds,
	}, nil
}

// ExecuteCode executes code in a sandboxed environment
func (s *Sandbox) ExecuteCode(ctx context.Context, language string, code string, input map[string]interface{}) (map[string]interface{}, error) {
	switch language {
	case "javascript", "js":
		return s.executeJavaScript(ctx, code, input)
	case "python", "py":
		return s.executePython(ctx, code, input)
	case "shell", "bash", "sh":
		return s.executeShell(ctx, code, input)
	default:
		return nil, fmt.Errorf("unsupported language: %s", language)
	}
}

// executeJavaScript executes JavaScript code using Node.js
func (s *Sandbox) executeJavaScript(ctx context.Context, code string, input map[string]interface{}) (map[string]interface{}, error) {
	// Create temporary file for the script
	tempFile := filepath.Join(s.tempDir, fmt.Sprintf("script_%d.js", time.Now().UnixNano()))

	// Wrap the code with input/output handling
	wrappedCode := s.wrapJavaScriptCode(code, input)

	if err := os.WriteFile(tempFile, []byte(wrappedCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write script file: %w", err)
	}
	defer os.Remove(tempFile)

	// Execute with timeout
	ctx, cancel := context.WithTimeout(ctx, s.maxExecTime)
	defer cancel()

	cmd := exec.CommandContext(ctx, "node", tempFile)

	// Set resource limits
	s.setResourceLimits(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("script execution failed: %w, output: %s", err, string(output))
	}

	// Parse output JSON
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		// If output is not JSON, return as string
		return map[string]interface{}{
			"output": string(output),
		}, nil
	}

	return result, nil
}

// executePython executes Python code
func (s *Sandbox) executePython(ctx context.Context, code string, input map[string]interface{}) (map[string]interface{}, error) {
	// Create temporary file for the script
	tempFile := filepath.Join(s.tempDir, fmt.Sprintf("script_%d.py", time.Now().UnixNano()))

	// Wrap the code with input/output handling
	wrappedCode := s.wrapPythonCode(code, input)

	if err := os.WriteFile(tempFile, []byte(wrappedCode), 0644); err != nil {
		return nil, fmt.Errorf("failed to write script file: %w", err)
	}
	defer os.Remove(tempFile)

	// Execute with timeout
	ctx, cancel := context.WithTimeout(ctx, s.maxExecTime)
	defer cancel()

	cmd := exec.CommandContext(ctx, "python3", tempFile)

	// Set resource limits
	s.setResourceLimits(cmd)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("script execution failed: %w, output: %s", err, string(output))
	}

	// Parse output JSON
	var result map[string]interface{}
	if err := json.Unmarshal(output, &result); err != nil {
		// If output is not JSON, return as string
		return map[string]interface{}{
			"output": string(output),
		}, nil
	}

	return result, nil
}

// executeShell executes shell commands (restricted)
func (s *Sandbox) executeShell(ctx context.Context, code string, input map[string]interface{}) (map[string]interface{}, error) {
	// Validate command is allowed
	if !s.isCommandAllowed(code) {
		return nil, fmt.Errorf("command not allowed for security reasons")
	}

	// Execute with timeout
	ctx, cancel := context.WithTimeout(ctx, s.maxExecTime)
	defer cancel()

	// Create shell script with input variables
	script := s.wrapShellCode(code, input)

	cmd := exec.CommandContext(ctx, "sh", "-c", script)

	// Set resource limits
	s.setResourceLimits(cmd)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	result := map[string]interface{}{
		"stdout": stdout.String(),
		"stderr": stderr.String(),
	}

	if err != nil {
		result["error"] = err.Error()
		result["success"] = false
	} else {
		result["success"] = true
	}

	return result, nil
}

// wrapJavaScriptCode wraps user code with input/output handling
func (s *Sandbox) wrapJavaScriptCode(code string, input map[string]interface{}) string {
	inputJSON, _ := json.Marshal(input)

	return fmt.Sprintf(`
const input = %s;

// User code starts here
%s
// User code ends here

// Output the result
if (typeof result !== 'undefined') {
    console.log(JSON.stringify(result));
} else {
    console.log(JSON.stringify({ error: "No result variable defined" }));
}
`, string(inputJSON), code)
}

// wrapPythonCode wraps user code with input/output handling
func (s *Sandbox) wrapPythonCode(code string, input map[string]interface{}) string {
	inputJSON, _ := json.Marshal(input)

	return fmt.Sprintf(`
import json
import sys

input_data = json.loads('%s')

# User code starts here
%s
# User code ends here

# Output the result
try:
    print(json.dumps(result))
except NameError:
    print(json.dumps({"error": "No result variable defined"}))
`, strings.ReplaceAll(string(inputJSON), "'", "\\'"), code)
}

// wrapShellCode wraps shell code with input variables
func (s *Sandbox) wrapShellCode(code string, input map[string]interface{}) string {
	var envVars []string
	for k, v := range input {
		// Convert key to uppercase and replace non-alphanumeric with underscore
		key := strings.ToUpper(k)
		key = strings.ReplaceAll(key, "-", "_")

		// Convert value to string
		value := fmt.Sprintf("%v", v)
		envVars = append(envVars, fmt.Sprintf("%s='%s'", key, value))
	}

	return fmt.Sprintf("%s\n%s", strings.Join(envVars, "\n"), code)
}

// setResourceLimits sets resource limits for the command
func (s *Sandbox) setResourceLimits(cmd *exec.Cmd) {
	// Note: Setting actual resource limits requires platform-specific code
	// This is a simplified version

	// Set environment variables to limit resources
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("NODE_OPTIONS=--max-old-space-size=%d", s.maxMemory/(1024*1024)),
		"PYTHONUNBUFFERED=1",
	)
}

// isCommandAllowed checks if a shell command is allowed
func (s *Sandbox) isCommandAllowed(command string) bool {
	// List of dangerous commands that should not be allowed
	dangerousCommands := []string{
		"rm", "dd", "mkfs", "format", "fdisk",
		"kill", "pkill", "killall",
		"shutdown", "reboot", "halt",
		"chmod", "chown",
		"useradd", "userdel", "passwd",
		"iptables", "firewall",
		"systemctl", "service",
	}

	// Check for dangerous commands
	lowerCommand := strings.ToLower(command)
	for _, dangerous := range dangerousCommands {
		if strings.Contains(lowerCommand, dangerous) {
			return false
		}
	}

	// Check for allowed commands if specified
	if len(s.allowedCmds) > 0 {
		allowed := false
		for _, allowedCmd := range s.allowedCmds {
			if strings.HasPrefix(command, allowedCmd) {
				allowed = true
				break
			}
		}
		return allowed
	}

	return true
}

// Cleanup cleans up temporary files
func (s *Sandbox) Cleanup() error {
	// Clean up old temporary files
	files, err := filepath.Glob(filepath.Join(s.tempDir, "script_*"))
	if err != nil {
		return err
	}

	for _, file := range files {
		info, err := os.Stat(file)
		if err != nil {
			continue
		}

		// Remove files older than 1 hour
		if time.Since(info.ModTime()) > time.Hour {
			os.Remove(file)
		}
	}

	return nil
}

// ValidateCode performs basic validation on the code
func (s *Sandbox) ValidateCode(language, code string) error {
	if code == "" {
		return fmt.Errorf("code cannot be empty")
	}

	// Add language-specific validation
	switch language {
	case "javascript", "js":
		// Check for obvious infinite loops
		if strings.Contains(code, "while(true)") || strings.Contains(code, "while (true)") {
			return fmt.Errorf("infinite loops are not allowed")
		}
	case "python", "py":
		// Check for dangerous imports
		dangerousImports := []string{"os", "subprocess", "sys", "eval", "exec", "__import__"}
		for _, imp := range dangerousImports {
			if strings.Contains(code, "import "+imp) || strings.Contains(code, "from "+imp) {
				return fmt.Errorf("import of %s is not allowed", imp)
			}
		}
	case "shell", "bash", "sh":
		if !s.isCommandAllowed(code) {
			return fmt.Errorf("command contains restricted operations")
		}
	}

	return nil
}

// ExecuteWithStreaming executes code and streams output
func (s *Sandbox) ExecuteWithStreaming(ctx context.Context, language, code string, input map[string]interface{}, output io.Writer) error {
	// This would implement streaming execution for long-running scripts
	// For now, just execute normally and write to output
	result, err := s.ExecuteCode(ctx, language, code, input)
	if err != nil {
		return err
	}

	jsonResult, _ := json.Marshal(result)
	_, writeErr := output.Write(jsonResult)
	return writeErr
}
