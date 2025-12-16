package resolver

import (
	"context"
	"time"
)

// ExecutionUpdated subscribes to execution updates
func (r *subscriptionResolver) ExecutionUpdated(ctx context.Context, executionID string) (<-chan *ExecutionUpdate, error) {
	ch := make(chan *ExecutionUpdate, 10)

	go func() {
		defer close(ch)

		// Poll for updates (in production, use WebSocket or SSE)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Fetch execution status
				execution, err := r.Query().Execution(ctx, executionID)
				if err != nil {
					continue
				}

				update := &ExecutionUpdate{
					ExecutionID: executionID,
					Status:      execution.Status,
					Timestamp:   time.Now(),
				}

				select {
				case ch <- update:
				case <-ctx.Done():
					return
				}

				// Stop if execution is complete
				if execution.Status == ExecutionStatusCompleted ||
					execution.Status == ExecutionStatusFailed ||
					execution.Status == ExecutionStatusCancelled {
					return
				}
			}
		}
	}()

	return ch, nil
}

// WorkflowExecutions subscribes to workflow executions
func (r *subscriptionResolver) WorkflowExecutions(ctx context.Context, workflowID string) (<-chan *Execution, error) {
	ch := make(chan *Execution, 10)

	go func() {
		defer close(ch)

		// Poll for new executions (in production, use event streaming)
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		var lastExecutionID string

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Fetch latest executions for workflow
				filter := &ExecutionFilter{WorkflowID: &workflowID}
				executions, err := r.Query().Executions(ctx, filter, &PaginationInput{First: intPtr(1)})
				if err != nil || len(executions.Edges) == 0 {
					continue
				}

				latest := executions.Edges[0].Node
				if latest.ID != lastExecutionID {
					lastExecutionID = latest.ID
					select {
					case ch <- latest:
					case <-ctx.Done():
						return
					}
				}
			}
		}
	}()

	return ch, nil
}

// Notifications subscribes to user notifications
func (r *subscriptionResolver) Notifications(ctx context.Context) (<-chan *Notification, error) {
	ch := make(chan *Notification, 10)

	go func() {
		defer close(ch)

		// In production, connect to notification service via WebSocket
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				// Placeholder - in production, receive from notification service
			}
		}
	}()

	return ch, nil
}

func intPtr(i int) *int {
	return &i
}
