package cost

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/linkflow-go/internal/domain/workflow"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
)

// Calculator calculates execution costs
type Calculator struct {
	mu            sync.RWMutex
	costModel     CostModel
	pricingRules  map[string]PricingRule
	usageTracker  *UsageTracker
	eventBus      events.EventBus
	logger        logger.Logger
	
	// Cost aggregations
	executionCosts map[string]*ExecutionCost
	userCosts      map[string]*UserCost
	teamCosts      map[string]*TeamCost
	
	// Metrics
	totalCostCalculated float64
	totalExecutions     int64
}

// CostModel defines the cost model for execution
type CostModel struct {
	ComputeCostPerSecond  float64 `json:"compute_cost_per_second"`
	MemoryCostPerGB       float64 `json:"memory_cost_per_gb"`
	StorageCostPerGB      float64 `json:"storage_cost_per_gb"`
	NetworkCostPerGB      float64 `json:"network_cost_per_gb"`
	APICallCost           float64 `json:"api_call_cost"`
	DatabaseQueryCost     float64 `json:"database_query_cost"`
	
	// Node-specific costs
	NodeTypeCosts map[string]float64 `json:"node_type_costs"`
	
	// Tier-based pricing
	TierDiscounts []TierDiscount `json:"tier_discounts"`
	
	// Currency
	Currency string `json:"currency"`
}

// TierDiscount represents a discount tier
type TierDiscount struct {
	MinUsage float64 `json:"min_usage"`
	MaxUsage float64 `json:"max_usage"`
	Discount float64 `json:"discount"` // Percentage discount
}

// PricingRule defines custom pricing rules
type PricingRule interface {
	Apply(usage ResourceUsage, baseCost float64) float64
	Name() string
}

// ExecutionCost represents the cost of an execution
type ExecutionCost struct {
	ExecutionID      string           `json:"execution_id"`
	WorkflowID       string           `json:"workflow_id"`
	UserID           string           `json:"user_id"`
	TeamID           string           `json:"team_id"`
	
	// Resource usage
	ComputeTime      time.Duration    `json:"compute_time"`
	MemoryUsageGB    float64          `json:"memory_usage_gb"`
	StorageUsageGB   float64          `json:"storage_usage_gb"`
	NetworkUsageGB   float64          `json:"network_usage_gb"`
	APICallCount     int              `json:"api_call_count"`
	DatabaseQueries  int              `json:"database_queries"`
	
	// Node costs
	NodeCosts        map[string]float64 `json:"node_costs"`
	
	// Cost breakdown
	ComputeCost      float64          `json:"compute_cost"`
	MemoryCost       float64          `json:"memory_cost"`
	StorageCost      float64          `json:"storage_cost"`
	NetworkCost      float64          `json:"network_cost"`
	APICallCost      float64          `json:"api_call_cost"`
	DatabaseCost     float64          `json:"database_cost"`
	
	// Total
	SubTotal         float64          `json:"subtotal"`
	Discount         float64          `json:"discount"`
	TotalCost        float64          `json:"total_cost"`
	
	// Metadata
	StartTime        time.Time        `json:"start_time"`
	EndTime          *time.Time       `json:"end_time,omitempty"`
	CalculatedAt     time.Time        `json:"calculated_at"`
}

// UserCost represents aggregated costs for a user
type UserCost struct {
	UserID           string           `json:"user_id"`
	Period           string           `json:"period"` // daily, weekly, monthly
	TotalExecutions  int              `json:"total_executions"`
	TotalCost        float64          `json:"total_cost"`
	AverageCost      float64          `json:"average_cost"`
	
	// Breakdown by workflow
	WorkflowCosts    map[string]float64 `json:"workflow_costs"`
	
	// Resource totals
	TotalComputeTime time.Duration    `json:"total_compute_time"`
	TotalMemoryGB    float64          `json:"total_memory_gb"`
	TotalStorageGB   float64          `json:"total_storage_gb"`
	TotalNetworkGB   float64          `json:"total_network_gb"`
}

// TeamCost represents aggregated costs for a team
type TeamCost struct {
	TeamID           string           `json:"team_id"`
	Period           string           `json:"period"`
	TotalUsers       int              `json:"total_users"`
	TotalExecutions  int              `json:"total_executions"`
	TotalCost        float64          `json:"total_cost"`
	
	// Breakdown by user
	UserCosts        map[string]float64 `json:"user_costs"`
	
	// Top workflows by cost
	TopWorkflows     []WorkflowCostInfo `json:"top_workflows"`
}

// WorkflowCostInfo represents cost information for a workflow
type WorkflowCostInfo struct {
	WorkflowID      string  `json:"workflow_id"`
	WorkflowName    string  `json:"workflow_name"`
	ExecutionCount  int     `json:"execution_count"`
	TotalCost       float64 `json:"total_cost"`
	AverageCost     float64 `json:"average_cost"`
}

// ResourceUsage represents resource usage for an execution
type ResourceUsage struct {
	ExecutionID     string
	ComputeTime     time.Duration
	MemoryBytes     int64
	StorageBytes    int64
	NetworkBytes    int64
	APICallCount    int
	DatabaseQueries int
}

// NewCalculator creates a new cost calculator
func NewCalculator(model CostModel, eventBus events.EventBus, logger logger.Logger) *Calculator {
	calc := &Calculator{
		costModel:      model,
		pricingRules:   make(map[string]PricingRule),
		usageTracker:   NewUsageTracker(logger),
		eventBus:       eventBus,
		logger:         logger,
		executionCosts: make(map[string]*ExecutionCost),
		userCosts:      make(map[string]*UserCost),
		teamCosts:      make(map[string]*TeamCost),
	}
	
	// Set defaults
	if calc.costModel.Currency == "" {
		calc.costModel.Currency = "USD"
	}
	
	// Register default pricing rules
	calc.registerDefaultRules()
	
	return calc
}

// registerDefaultRules registers default pricing rules
func (c *Calculator) registerDefaultRules() {
	c.RegisterPricingRule(&VolumDiscountRule{})
	c.RegisterPricingRule(&TimeOfDayRule{})
	c.RegisterPricingRule(&ResourceOptimizationRule{})
}

// Start starts the cost calculator
func (c *Calculator) Start(ctx context.Context) error {
	c.logger.Info("Starting cost calculator")
	
	// Subscribe to events
	if err := c.subscribeToEvents(ctx); err != nil {
		return err
	}
	
	// Start usage tracker
	return c.usageTracker.Start(ctx)
}

// Stop stops the cost calculator
func (c *Calculator) Stop(ctx context.Context) error {
	c.logger.Info("Stopping cost calculator")
	
	// Stop usage tracker
	return c.usageTracker.Stop(ctx)
}

// RegisterPricingRule registers a pricing rule
func (c *Calculator) RegisterPricingRule(rule PricingRule) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.pricingRules[rule.Name()] = rule
	c.logger.Info("Registered pricing rule", "name", rule.Name())
}

// CalculateExecutionCost calculates the cost for an execution
func (c *Calculator) CalculateExecutionCost(ctx context.Context, executionID string, usage ResourceUsage) (*ExecutionCost, error) {
	cost := &ExecutionCost{
		ExecutionID:  executionID,
		StartTime:    time.Now(),
		CalculatedAt: time.Now(),
		NodeCosts:    make(map[string]float64),
	}
	
	// Calculate resource costs
	cost.ComputeTime = usage.ComputeTime
	cost.ComputeCost = usage.ComputeTime.Seconds() * c.costModel.ComputeCostPerSecond
	
	cost.MemoryUsageGB = float64(usage.MemoryBytes) / (1024 * 1024 * 1024)
	cost.MemoryCost = cost.MemoryUsageGB * c.costModel.MemoryCostPerGB
	
	cost.StorageUsageGB = float64(usage.StorageBytes) / (1024 * 1024 * 1024)
	cost.StorageCost = cost.StorageUsageGB * c.costModel.StorageCostPerGB
	
	cost.NetworkUsageGB = float64(usage.NetworkBytes) / (1024 * 1024 * 1024)
	cost.NetworkCost = cost.NetworkUsageGB * c.costModel.NetworkCostPerGB
	
	cost.APICallCount = usage.APICallCount
	cost.APICallCost = float64(usage.APICallCount) * c.costModel.APICallCost
	
	cost.DatabaseQueries = usage.DatabaseQueries
	cost.DatabaseCost = float64(usage.DatabaseQueries) * c.costModel.DatabaseQueryCost
	
	// Calculate subtotal
	cost.SubTotal = cost.ComputeCost + cost.MemoryCost + cost.StorageCost +
		cost.NetworkCost + cost.APICallCost + cost.DatabaseCost
	
	// Apply pricing rules
	finalCost := cost.SubTotal
	for _, rule := range c.pricingRules {
		finalCost = rule.Apply(usage, finalCost)
	}
	
	// Apply tier discounts
	discount := c.calculateTierDiscount(finalCost)
	cost.Discount = discount
	
	// Calculate final cost
	cost.TotalCost = finalCost * (1 - discount)
	
	// Store cost
	c.mu.Lock()
	c.executionCosts[executionID] = cost
	c.totalCostCalculated += cost.TotalCost
	c.totalExecutions++
	c.mu.Unlock()
	
	// Publish cost event
	c.publishCostEvent(ctx, cost)
	
	c.logger.Info("Execution cost calculated",
		"executionId", executionID,
		"totalCost", cost.TotalCost,
		"currency", c.costModel.Currency,
	)
	
	return cost, nil
}

// calculateTierDiscount calculates tier-based discount
func (c *Calculator) calculateTierDiscount(cost float64) float64 {
	for _, tier := range c.costModel.TierDiscounts {
		if cost >= tier.MinUsage && (tier.MaxUsage == 0 || cost < tier.MaxUsage) {
			return tier.Discount / 100
		}
	}
	return 0
}

// GetExecutionCost gets the cost for an execution
func (c *Calculator) GetExecutionCost(executionID string) (*ExecutionCost, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	cost, exists := c.executionCosts[executionID]
	if !exists {
		return nil, fmt.Errorf("cost not found for execution: %s", executionID)
	}
	
	return cost, nil
}

// GetUserCost gets aggregated costs for a user
func (c *Calculator) GetUserCost(userID string, period string) (*UserCost, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Calculate user costs
	userCost := &UserCost{
		UserID:        userID,
		Period:        period,
		WorkflowCosts: make(map[string]float64),
	}
	
	for _, cost := range c.executionCosts {
		if cost.UserID == userID {
			userCost.TotalExecutions++
			userCost.TotalCost += cost.TotalCost
			userCost.TotalComputeTime += cost.ComputeTime
			userCost.TotalMemoryGB += cost.MemoryUsageGB
			userCost.TotalStorageGB += cost.StorageUsageGB
			userCost.TotalNetworkGB += cost.NetworkUsageGB
			
			// Aggregate by workflow
			userCost.WorkflowCosts[cost.WorkflowID] += cost.TotalCost
		}
	}
	
	if userCost.TotalExecutions > 0 {
		userCost.AverageCost = userCost.TotalCost / float64(userCost.TotalExecutions)
	}
	
	c.userCosts[userID] = userCost
	
	return userCost, nil
}

// GetTeamCost gets aggregated costs for a team
func (c *Calculator) GetTeamCost(teamID string, period string) (*TeamCost, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Calculate team costs
	teamCost := &TeamCost{
		TeamID:    teamID,
		Period:    period,
		UserCosts: make(map[string]float64),
	}
	
	workflowCosts := make(map[string]*WorkflowCostInfo)
	userSet := make(map[string]bool)
	
	for _, cost := range c.executionCosts {
		if cost.TeamID == teamID {
			teamCost.TotalExecutions++
			teamCost.TotalCost += cost.TotalCost
			
			// Track users
			userSet[cost.UserID] = true
			teamCost.UserCosts[cost.UserID] += cost.TotalCost
			
			// Aggregate workflow costs
			if info, exists := workflowCosts[cost.WorkflowID]; exists {
				info.ExecutionCount++
				info.TotalCost += cost.TotalCost
			} else {
				workflowCosts[cost.WorkflowID] = &WorkflowCostInfo{
					WorkflowID:     cost.WorkflowID,
					ExecutionCount: 1,
					TotalCost:      cost.TotalCost,
				}
			}
		}
	}
	
	teamCost.TotalUsers = len(userSet)
	
	// Calculate averages and find top workflows
	for _, info := range workflowCosts {
		info.AverageCost = info.TotalCost / float64(info.ExecutionCount)
		teamCost.TopWorkflows = append(teamCost.TopWorkflows, *info)
	}
	
	// Sort top workflows by cost
	// In production, would use proper sorting
	
	c.teamCosts[teamID] = teamCost
	
	return teamCost, nil
}

// GenerateOptimizationSuggestions generates cost optimization suggestions
func (c *Calculator) GenerateOptimizationSuggestions(executionID string) []OptimizationSuggestion {
	c.mu.RLock()
	cost, exists := c.executionCosts[executionID]
	c.mu.RUnlock()
	
	if !exists {
		return nil
	}
	
	suggestions := []OptimizationSuggestion{}
	
	// Check compute time optimization
	if cost.ComputeCost > cost.TotalCost*0.5 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "compute",
			Description: "High compute costs detected",
			Suggestion:  "Consider optimizing algorithms or using more efficient node types",
			Savings:     cost.ComputeCost * 0.2, // Estimated 20% savings
		})
	}
	
	// Check memory usage
	if cost.MemoryCost > cost.TotalCost*0.3 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "memory",
			Description: "High memory usage detected",
			Suggestion:  "Optimize data structures and reduce memory footprint",
			Savings:     cost.MemoryCost * 0.25,
		})
	}
	
	// Check API calls
	if cost.APICallCount > 1000 {
		suggestions = append(suggestions, OptimizationSuggestion{
			Type:        "api",
			Description: "High number of API calls",
			Suggestion:  "Batch API calls where possible",
			Savings:     cost.APICallCost * 0.3,
		})
	}
	
	return suggestions
}

// publishCostEvent publishes a cost calculation event
func (c *Calculator) publishCostEvent(ctx context.Context, cost *ExecutionCost) {
	event := events.NewEventBuilder("cost.calculated").
		WithAggregateID(cost.ExecutionID).
		WithPayload("cost", cost).
		Build()
	
	c.eventBus.Publish(ctx, event)
}

// subscribeToEvents subscribes to relevant events
func (c *Calculator) subscribeToEvents(ctx context.Context) error {
	events := map[string]events.HandlerFunc{
		events.ExecutionCompleted: c.handleExecutionCompleted,
		"resource.usage.reported": c.handleResourceUsage,
	}
	
	for eventType, handler := range events {
		if err := c.eventBus.Subscribe(eventType, handler); err != nil {
			return err
		}
	}
	
	return nil
}

// Event handlers

func (c *Calculator) handleExecutionCompleted(ctx context.Context, event events.Event) error {
	executionID := event.AggregateID
	
	// Get resource usage from tracker
	usage, err := c.usageTracker.GetUsage(executionID)
	if err != nil {
		return err
	}
	
	// Calculate cost
	_, err = c.CalculateExecutionCost(ctx, executionID, *usage)
	return err
}

func (c *Calculator) handleResourceUsage(ctx context.Context, event events.Event) error {
	executionID, _ := event.Payload["executionId"].(string)
	
	// Track resource usage
	usage := ResourceUsage{
		ExecutionID: executionID,
		// Extract usage from event payload
	}
	
	return c.usageTracker.TrackUsage(executionID, usage)
}

// GetMetrics returns cost calculator metrics
func (c *Calculator) GetMetrics() CostMetrics {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	avgCost := float64(0)
	if c.totalExecutions > 0 {
		avgCost = c.totalCostCalculated / float64(c.totalExecutions)
	}
	
	return CostMetrics{
		TotalCostCalculated: c.totalCostCalculated,
		TotalExecutions:     c.totalExecutions,
		AverageCost:         avgCost,
		Currency:            c.costModel.Currency,
	}
}

// OptimizationSuggestion represents a cost optimization suggestion
type OptimizationSuggestion struct {
	Type        string  `json:"type"`
	Description string  `json:"description"`
	Suggestion  string  `json:"suggestion"`
	Savings     float64 `json:"estimated_savings"`
}

// CostMetrics contains cost calculator metrics
type CostMetrics struct {
	TotalCostCalculated float64 `json:"total_cost_calculated"`
	TotalExecutions     int64   `json:"total_executions"`
	AverageCost         float64 `json:"average_cost"`
	Currency            string  `json:"currency"`
}

// Pricing Rules

// VolumDiscountRule applies volume-based discounts
type VolumDiscountRule struct{}

func (r *VolumDiscountRule) Apply(usage ResourceUsage, baseCost float64) float64 {
	// Apply volume discount for high usage
	if usage.ComputeTime > 1*time.Hour {
		return baseCost * 0.9 // 10% discount
	}
	return baseCost
}

func (r *VolumDiscountRule) Name() string {
	return "volume_discount"
}

// TimeOfDayRule applies time-of-day pricing
type TimeOfDayRule struct{}

func (r *TimeOfDayRule) Apply(usage ResourceUsage, baseCost float64) float64 {
	hour := time.Now().Hour()
	
	// Off-peak hours (midnight to 6am)
	if hour >= 0 && hour < 6 {
		return baseCost * 0.8 // 20% discount
	}
	
	// Peak hours (9am to 5pm)
	if hour >= 9 && hour < 17 {
		return baseCost * 1.1 // 10% premium
	}
	
	return baseCost
}

func (r *TimeOfDayRule) Name() string {
	return "time_of_day"
}

// ResourceOptimizationRule rewards efficient resource usage
type ResourceOptimizationRule struct{}

func (r *ResourceOptimizationRule) Apply(usage ResourceUsage, baseCost float64) float64 {
	// Calculate resource efficiency
	memoryEfficiency := float64(usage.MemoryBytes) / float64(usage.ComputeTime.Seconds()+1)
	
	// Reward efficient memory usage
	if memoryEfficiency < 1024*1024 { // Less than 1MB per second
		return baseCost * 0.95 // 5% discount for efficiency
	}
	
	return baseCost
}

func (r *ResourceOptimizationRule) Name() string {
	return "resource_optimization"
}
