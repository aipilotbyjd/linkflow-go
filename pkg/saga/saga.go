package saga

import (
	"context"
	"fmt"
	"sync"
)

// Step represents a single step in a saga
type Step struct {
	Name         string
	Action       func(ctx context.Context, data interface{}) error
	Compensation func(ctx context.Context, data interface{}) error
}

// Orchestrator manages saga execution
type Orchestrator struct {
	steps          []Step
	completedSteps []Step
	mu             sync.Mutex
}

// NewOrchestrator creates a new saga orchestrator
func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		steps:          []Step{},
		completedSteps: []Step{},
	}
}

// AddStep adds a step to the saga
func (o *Orchestrator) AddStep(step *Step) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.steps = append(o.steps, *step)
}

// Execute runs the saga
func (o *Orchestrator) Execute(ctx context.Context, data interface{}) error {
	o.mu.Lock()
	defer o.mu.Unlock()

	// Reset completed steps
	o.completedSteps = []Step{}

	// Execute each step
	for _, step := range o.steps {
		// Check context cancellation
		select {
		case <-ctx.Done():
			// Context cancelled, run compensations
			return o.compensate(ctx, data)
		default:
		}

		// Execute step action
		if err := step.Action(ctx, data); err != nil {
			// Step failed, run compensations
			compensateErr := o.compensate(ctx, data)
			if compensateErr != nil {
				return fmt.Errorf("step '%s' failed: %w, compensation also failed: %v",
					step.Name, err, compensateErr)
			}
			return fmt.Errorf("step '%s' failed: %w", step.Name, err)
		}

		// Mark step as completed
		o.completedSteps = append(o.completedSteps, step)
	}

	return nil
}

// compensate runs compensation for completed steps in reverse order
func (o *Orchestrator) compensate(ctx context.Context, data interface{}) error {
	var compensationErrors []error

	// Run compensations in reverse order
	for i := len(o.completedSteps) - 1; i >= 0; i-- {
		step := o.completedSteps[i]

		if step.Compensation != nil {
			if err := step.Compensation(ctx, data); err != nil {
				compensationErrors = append(compensationErrors,
					fmt.Errorf("compensation for step '%s' failed: %w", step.Name, err))
			}
		}
	}

	if len(compensationErrors) > 0 {
		return fmt.Errorf("compensation errors: %v", compensationErrors)
	}

	return nil
}

// Transaction represents a distributed transaction
type Transaction struct {
	ID           string
	Participants []Participant
	State        TransactionState
	mu           sync.Mutex
}

// Participant in a distributed transaction
type Participant struct {
	ID        string
	Service   string
	Prepared  bool
	Committed bool
}

// TransactionState represents the state of a transaction
type TransactionState int

const (
	StateInitial TransactionState = iota
	StatePreparing
	StatePrepared
	StateCommitting
	StateCommitted
	StateAborting
	StateAborted
)

// TwoPhaseCommit implements 2PC protocol
type TwoPhaseCommit struct {
	transactions map[string]*Transaction
	mu           sync.RWMutex
}

// NewTwoPhaseCommit creates a new 2PC coordinator
func NewTwoPhaseCommit() *TwoPhaseCommit {
	return &TwoPhaseCommit{
		transactions: make(map[string]*Transaction),
	}
}

// Begin starts a new transaction
func (tpc *TwoPhaseCommit) Begin(transactionID string, participants []string) *Transaction {
	tpc.mu.Lock()
	defer tpc.mu.Unlock()

	transaction := &Transaction{
		ID:           transactionID,
		Participants: make([]Participant, len(participants)),
		State:        StateInitial,
	}

	for i, p := range participants {
		transaction.Participants[i] = Participant{
			ID:      fmt.Sprintf("%s-%d", transactionID, i),
			Service: p,
		}
	}

	tpc.transactions[transactionID] = transaction
	return transaction
}

// Prepare asks all participants to prepare
func (tpc *TwoPhaseCommit) Prepare(ctx context.Context, transactionID string) error {
	tpc.mu.RLock()
	transaction, exists := tpc.transactions[transactionID]
	tpc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("transaction not found: %s", transactionID)
	}

	transaction.mu.Lock()
	defer transaction.mu.Unlock()

	transaction.State = StatePreparing

	// Ask all participants to prepare
	for i := range transaction.Participants {
		// In a real implementation, this would call the participant service
		transaction.Participants[i].Prepared = true
	}

	// Check if all participants are prepared
	allPrepared := true
	for _, p := range transaction.Participants {
		if !p.Prepared {
			allPrepared = false
			break
		}
	}

	if allPrepared {
		transaction.State = StatePrepared
		return nil
	}

	// Some participants failed to prepare, abort
	transaction.State = StateAborting
	return fmt.Errorf("some participants failed to prepare")
}

// Commit commits the transaction
func (tpc *TwoPhaseCommit) Commit(ctx context.Context, transactionID string) error {
	tpc.mu.RLock()
	transaction, exists := tpc.transactions[transactionID]
	tpc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("transaction not found: %s", transactionID)
	}

	transaction.mu.Lock()
	defer transaction.mu.Unlock()

	if transaction.State != StatePrepared {
		return fmt.Errorf("transaction not in prepared state")
	}

	transaction.State = StateCommitting

	// Tell all participants to commit
	for i := range transaction.Participants {
		// In a real implementation, this would call the participant service
		transaction.Participants[i].Committed = true
	}

	transaction.State = StateCommitted
	return nil
}

// Abort aborts the transaction
func (tpc *TwoPhaseCommit) Abort(ctx context.Context, transactionID string) error {
	tpc.mu.RLock()
	transaction, exists := tpc.transactions[transactionID]
	tpc.mu.RUnlock()

	if !exists {
		return fmt.Errorf("transaction not found: %s", transactionID)
	}

	transaction.mu.Lock()
	defer transaction.mu.Unlock()

	transaction.State = StateAborting

	// Tell all participants to abort
	for i := range transaction.Participants {
		// In a real implementation, this would call the participant service
		transaction.Participants[i].Prepared = false
		transaction.Participants[i].Committed = false
	}

	transaction.State = StateAborted
	return nil
}
