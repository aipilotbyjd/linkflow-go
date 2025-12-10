package types

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	_ "github.com/lib/pq"           // PostgreSQL driver
	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/linkflow-go/pkg/logger"
)

// DatabaseNodeExecutor handles database query nodes
type DatabaseNodeExecutor struct {
	logger      logger.Logger
	connections map[string]*sql.DB
}

// DatabaseNodeConfig represents configuration for database nodes
type DatabaseNodeConfig struct {
	ConnectionString string                 `json:"connectionString"`
	DatabaseType     string                 `json:"databaseType"` // postgres, mysql, sqlite
	Query            string                 `json:"query"`
	Parameters       []interface{}          `json:"parameters"`
	Operation        string                 `json:"operation"` // select, insert, update, delete
	Transaction      bool                   `json:"transaction"`
	MaxRows          int                    `json:"maxRows"`
	Timeout          int                    `json:"timeout"` // in seconds
}

// NewDatabaseNodeExecutor creates a new database node executor
func NewDatabaseNodeExecutor(logger logger.Logger) *DatabaseNodeExecutor {
	return &DatabaseNodeExecutor{
		logger:      logger,
		connections: make(map[string]*sql.DB),
	}
}

// Execute executes a database query node
func (e *DatabaseNodeExecutor) Execute(ctx context.Context, node Node, input map[string]interface{}) (map[string]interface{}, error) {
	config, err := e.parseConfig(node.Config)
	if err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}
	
	// Get or create database connection
	db, err := e.getConnection(config)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}
	
	// Interpolate variables in query
	query := e.interpolateVariables(config.Query, input)
	
	// Set timeout
	if config.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(config.Timeout)*time.Second)
		defer cancel()
	}
	
	// Execute based on operation type
	switch strings.ToLower(config.Operation) {
	case "select":
		return e.executeSelect(ctx, db, query, config.Parameters, config.MaxRows)
	case "insert", "update", "delete":
		return e.executeModification(ctx, db, query, config.Parameters, config.Transaction)
	default:
		// Auto-detect operation from query
		if e.isSelectQuery(query) {
			return e.executeSelect(ctx, db, query, config.Parameters, config.MaxRows)
		}
		return e.executeModification(ctx, db, query, config.Parameters, config.Transaction)
	}
}

// ValidateInput validates the input for the database node
func (e *DatabaseNodeExecutor) ValidateInput(node Node, input map[string]interface{}) error {
	config, err := e.parseConfig(node.Config)
	if err != nil {
		return err
	}
	
	if config.ConnectionString == "" {
		return fmt.Errorf("database connection string is required")
	}
	
	if config.Query == "" {
		return fmt.Errorf("query is required")
	}
	
	// Validate database type
	validTypes := []string{"postgres", "mysql", "sqlite", "mssql"}
	valid := false
	for _, t := range validTypes {
		if config.DatabaseType == t {
			valid = true
			break
		}
	}
	
	if !valid {
		return fmt.Errorf("invalid database type: %s", config.DatabaseType)
	}
	
	return nil
}

// GetTimeout returns the timeout for the database operation
func (e *DatabaseNodeExecutor) GetTimeout() time.Duration {
	return 30 * time.Second
}

// parseConfig parses the node configuration
func (e *DatabaseNodeExecutor) parseConfig(config interface{}) (*DatabaseNodeConfig, error) {
	jsonData, err := json.Marshal(config)
	if err != nil {
		return nil, err
	}
	
	var dbConfig DatabaseNodeConfig
	if err := json.Unmarshal(jsonData, &dbConfig); err != nil {
		return nil, err
	}
	
	// Set defaults
	if dbConfig.MaxRows == 0 {
		dbConfig.MaxRows = 1000
	}
	
	if dbConfig.Timeout == 0 {
		dbConfig.Timeout = 30
	}
	
	return &dbConfig, nil
}

// getConnection gets or creates a database connection
func (e *DatabaseNodeExecutor) getConnection(config *DatabaseNodeConfig) (*sql.DB, error) {
	// Check if connection exists
	if db, exists := e.connections[config.ConnectionString]; exists {
		// Verify connection is still alive
		if err := db.Ping(); err == nil {
			return db, nil
		}
		// Connection is dead, remove it
		delete(e.connections, config.ConnectionString)
		db.Close()
	}
	
	// Create new connection
	var driverName string
	switch config.DatabaseType {
	case "postgres":
		driverName = "postgres"
	case "mysql":
		driverName = "mysql"
	case "sqlite":
		driverName = "sqlite3"
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.DatabaseType)
	}
	
	db, err := sql.Open(driverName, config.ConnectionString)
	if err != nil {
		return nil, err
	}
	
	// Test connection
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, err
	}
	
	// Configure connection pool
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)
	
	// Store connection
	e.connections[config.ConnectionString] = db
	
	return db, nil
}

// executeSelect executes a SELECT query
func (e *DatabaseNodeExecutor) executeSelect(ctx context.Context, db *sql.DB, query string, params []interface{}, maxRows int) (map[string]interface{}, error) {
	rows, err := db.QueryContext(ctx, query, params...)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	defer rows.Close()
	
	// Get column names
	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	
	// Prepare result
	var results []map[string]interface{}
	count := 0
	
	for rows.Next() && count < maxRows {
		// Create a slice of interface{} to hold column values
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		
		// Scan row
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, fmt.Errorf("failed to scan row: %w", err)
		}
		
		// Create row map
		row := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			// Handle NULL values
			if val == nil {
				row[col] = nil
			} else {
				// Convert byte arrays to strings
				if b, ok := val.([]byte); ok {
					row[col] = string(b)
				} else {
					row[col] = val
				}
			}
		}
		
		results = append(results, row)
		count++
	}
	
	return map[string]interface{}{
		"rows":     results,
		"rowCount": count,
		"columns":  columns,
		"success":  true,
	}, nil
}

// executeModification executes INSERT, UPDATE, or DELETE query
func (e *DatabaseNodeExecutor) executeModification(ctx context.Context, db *sql.DB, query string, params []interface{}, useTransaction bool) (map[string]interface{}, error) {
	var result sql.Result
	var err error
	
	if useTransaction {
		// Execute in transaction
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to begin transaction: %w", err)
		}
		
		result, err = tx.ExecContext(ctx, query, params...)
		if err != nil {
			tx.Rollback()
			return nil, fmt.Errorf("query execution failed: %w", err)
		}
		
		if err := tx.Commit(); err != nil {
			return nil, fmt.Errorf("failed to commit transaction: %w", err)
		}
	} else {
		// Execute without transaction
		result, err = db.ExecContext(ctx, query, params...)
		if err != nil {
			return nil, fmt.Errorf("query execution failed: %w", err)
		}
	}
	
	// Get affected rows
	rowsAffected, _ := result.RowsAffected()
	
	// Get last insert ID (if supported)
	lastInsertID, _ := result.LastInsertId()
	
	return map[string]interface{}{
		"rowsAffected": rowsAffected,
		"lastInsertId": lastInsertID,
		"success":      true,
	}, nil
}

// isSelectQuery checks if a query is a SELECT query
func (e *DatabaseNodeExecutor) isSelectQuery(query string) bool {
	trimmed := strings.TrimSpace(strings.ToUpper(query))
	return strings.HasPrefix(trimmed, "SELECT") || 
	       strings.HasPrefix(trimmed, "WITH") || 
	       strings.HasPrefix(trimmed, "SHOW") ||
	       strings.HasPrefix(trimmed, "DESCRIBE")
}

// interpolateVariables replaces {{variable}} with actual values
func (e *DatabaseNodeExecutor) interpolateVariables(template string, variables map[string]interface{}) string {
	result := template
	for key, value := range variables {
		placeholder := fmt.Sprintf("{{%s}}", key)
		result = strings.ReplaceAll(result, placeholder, fmt.Sprintf("%v", value))
	}
	return result
}

// Close closes all database connections
func (e *DatabaseNodeExecutor) Close() error {
	for _, db := range e.connections {
		if err := db.Close(); err != nil {
			e.logger.Error("Failed to close database connection", "error", err)
		}
	}
	e.connections = make(map[string]*sql.DB)
	return nil
}
