package ports

import "context"

type AuditRepository interface {
	GetAuditLogs(ctx context.Context, filters map[string]interface{}) ([]interface{}, error)
}
