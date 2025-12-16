package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/pkg/contracts/workflow"
	"github.com/linkflow-go/pkg/database"
	"gorm.io/gorm"
)

// WorkflowVersionRepository handles workflow version operations
type WorkflowVersionRepository struct {
	db *database.DB
}

// NewWorkflowVersionRepository creates a new workflow version repository
func NewWorkflowVersionRepository(db *database.DB) *WorkflowVersionRepository {
	return &WorkflowVersionRepository{db: db}
}

// Create creates a new workflow version
func (r *WorkflowVersionRepository) Create(ctx context.Context, wv *workflow.WorkflowVersion) error {
	return r.db.WithContext(ctx).Create(wv).Error
}

// CreateFromWorkflow creates a new version from a workflow
func (r *WorkflowVersionRepository) CreateFromWorkflow(ctx context.Context, w *workflow.Workflow, changeNote string) error {
	workflowJSON, err := w.ToJSON()
	if err != nil {
		return err
	}

	version := &workflow.WorkflowVersion{
		ID:         uuid.New().String(),
		WorkflowID: w.ID,
		Version:    w.Version,
		Data:       workflowJSON,
		ChangedBy:  w.UserID,
		ChangeNote: changeNote,
		CreatedAt:  time.Now(),
	}

	return r.Create(ctx, version)
}

// Get retrieves a specific version
func (r *WorkflowVersionRepository) Get(ctx context.Context, workflowID string, version int) (*workflow.WorkflowVersion, error) {
	var wv workflow.WorkflowVersion
	err := r.db.WithContext(ctx).
		Where("workflow_id = ? AND version = ?", workflowID, version).
		First(&wv).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("workflow version not found")
	}

	return &wv, err
}

// GetLatest retrieves the latest version of a workflow
func (r *WorkflowVersionRepository) GetLatest(ctx context.Context, workflowID string) (*workflow.WorkflowVersion, error) {
	var wv workflow.WorkflowVersion
	err := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("version DESC").
		First(&wv).Error

	if err == gorm.ErrRecordNotFound {
		return nil, fmt.Errorf("no versions found for workflow")
	}

	return &wv, err
}

// List lists all versions of a workflow
func (r *WorkflowVersionRepository) List(ctx context.Context, workflowID string, limit int) ([]*workflow.WorkflowVersion, error) {
	var versions []*workflow.WorkflowVersion

	query := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Order("version DESC")

	if limit > 0 {
		query = query.Limit(limit)
	}

	err := query.Find(&versions).Error
	return versions, err
}

// Compare compares two versions and returns the differences
func (r *WorkflowVersionRepository) Compare(ctx context.Context, workflowID string, version1, version2 int) (*VersionComparison, error) {
	v1, err := r.Get(ctx, workflowID, version1)
	if err != nil {
		return nil, err
	}

	v2, err := r.Get(ctx, workflowID, version2)
	if err != nil {
		return nil, err
	}

	var w1, w2 workflow.Workflow
	if err := json.Unmarshal([]byte(v1.Data), &w1); err != nil {
		return nil, err
	}

	if err := json.Unmarshal([]byte(v2.Data), &w2); err != nil {
		return nil, err
	}

	comparison := &VersionComparison{
		Version1:    version1,
		Version2:    version2,
		ChangedBy1:  v1.ChangedBy,
		ChangedBy2:  v2.ChangedBy,
		CreatedAt1:  v1.CreatedAt,
		CreatedAt2:  v2.CreatedAt,
		ChangeNote1: v1.ChangeNote,
		ChangeNote2: v2.ChangeNote,
	}

	// Compare basic fields
	if w1.Name != w2.Name {
		comparison.NameChanged = true
		comparison.OldName = w1.Name
		comparison.NewName = w2.Name
	}

	if w1.Description != w2.Description {
		comparison.DescriptionChanged = true
		comparison.OldDescription = w1.Description
		comparison.NewDescription = w2.Description
	}

	// Compare nodes
	comparison.NodesAdded = countAddedNodes(w1.Nodes, w2.Nodes)
	comparison.NodesRemoved = countRemovedNodes(w1.Nodes, w2.Nodes)
	comparison.NodesModified = countModifiedNodes(w1.Nodes, w2.Nodes)

	// Compare connections
	comparison.ConnectionsAdded = countAddedConnections(w1.Connections, w2.Connections)
	comparison.ConnectionsRemoved = countRemovedConnections(w1.Connections, w2.Connections)

	return comparison, nil
}

// Restore restores a specific version as the current version
func (r *WorkflowVersionRepository) Restore(ctx context.Context, workflowID string, versionToRestore int, userID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Get the version to restore
		var wv workflow.WorkflowVersion
		if err := tx.Where("workflow_id = ? AND version = ?", workflowID, versionToRestore).First(&wv).Error; err != nil {
			return err
		}

		// Parse the workflow data
		var restoredWorkflow workflow.Workflow
		if err := json.Unmarshal([]byte(wv.Data), &restoredWorkflow); err != nil {
			return err
		}

		// Get the latest version number
		var latestVersion int
		if err := tx.Model(&workflow.WorkflowVersion{}).
			Where("workflow_id = ?", workflowID).
			Select("MAX(version)").
			Scan(&latestVersion).Error; err != nil {
			return err
		}

		// Create a new version with the restored data
		newVersion := &workflow.WorkflowVersion{
			ID:         uuid.New().String(),
			WorkflowID: workflowID,
			Version:    latestVersion + 1,
			Data:       wv.Data,
			ChangedBy:  userID,
			ChangeNote: fmt.Sprintf("Restored from version %d", versionToRestore),
			CreatedAt:  time.Now(),
		}

		if err := tx.Create(newVersion).Error; err != nil {
			return err
		}

		// Update the workflow with the restored data
		restoredWorkflow.Version = newVersion.Version
		restoredWorkflow.UpdatedAt = time.Now()

		return tx.Save(&restoredWorkflow).Error
	})
}

// DeleteOldVersions deletes versions older than the specified date
func (r *WorkflowVersionRepository) DeleteOldVersions(ctx context.Context, workflowID string, keepLast int, olderThan time.Time) error {
	// Get versions to keep
	var versionsToKeep []int
	err := r.db.WithContext(ctx).
		Model(&workflow.WorkflowVersion{}).
		Where("workflow_id = ?", workflowID).
		Order("version DESC").
		Limit(keepLast).
		Pluck("version", &versionsToKeep).Error

	if err != nil {
		return err
	}

	// Delete old versions
	query := r.db.WithContext(ctx).
		Where("workflow_id = ?", workflowID).
		Where("created_at < ?", olderThan)

	if len(versionsToKeep) > 0 {
		query = query.Where("version NOT IN ?", versionsToKeep)
	}

	return query.Delete(&workflow.WorkflowVersion{}).Error
}

// GetVersionCount returns the number of versions for a workflow
func (r *WorkflowVersionRepository) GetVersionCount(ctx context.Context, workflowID string) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).
		Model(&workflow.WorkflowVersion{}).
		Where("workflow_id = ?", workflowID).
		Count(&count).Error

	return count, err
}

// VersionComparison represents the comparison between two versions
type VersionComparison struct {
	Version1           int       `json:"version1"`
	Version2           int       `json:"version2"`
	ChangedBy1         string    `json:"changedBy1"`
	ChangedBy2         string    `json:"changedBy2"`
	CreatedAt1         time.Time `json:"createdAt1"`
	CreatedAt2         time.Time `json:"createdAt2"`
	ChangeNote1        string    `json:"changeNote1"`
	ChangeNote2        string    `json:"changeNote2"`
	NameChanged        bool      `json:"nameChanged"`
	OldName            string    `json:"oldName,omitempty"`
	NewName            string    `json:"newName,omitempty"`
	DescriptionChanged bool      `json:"descriptionChanged"`
	OldDescription     string    `json:"oldDescription,omitempty"`
	NewDescription     string    `json:"newDescription,omitempty"`
	NodesAdded         int       `json:"nodesAdded"`
	NodesRemoved       int       `json:"nodesRemoved"`
	NodesModified      int       `json:"nodesModified"`
	ConnectionsAdded   int       `json:"connectionsAdded"`
	ConnectionsRemoved int       `json:"connectionsRemoved"`
}

// Helper functions for version comparison

func countAddedNodes(old, new []workflow.Node) int {
	oldMap := make(map[string]bool)
	for _, node := range old {
		oldMap[node.ID] = true
	}

	count := 0
	for _, node := range new {
		if !oldMap[node.ID] {
			count++
		}
	}
	return count
}

func countRemovedNodes(old, new []workflow.Node) int {
	newMap := make(map[string]bool)
	for _, node := range new {
		newMap[node.ID] = true
	}

	count := 0
	for _, node := range old {
		if !newMap[node.ID] {
			count++
		}
	}
	return count
}

func countModifiedNodes(old, new []workflow.Node) int {
	oldMap := make(map[string]workflow.Node)
	for _, node := range old {
		oldMap[node.ID] = node
	}

	count := 0
	for _, newNode := range new {
		if oldNode, exists := oldMap[newNode.ID]; exists {
			// Check if node has been modified
			if oldNode.Name != newNode.Name ||
				oldNode.Type != newNode.Type ||
				oldNode.Disabled != newNode.Disabled ||
				oldNode.RetryCount != newNode.RetryCount ||
				oldNode.Timeout != newNode.Timeout {
				count++
			}
		}
	}
	return count
}

func countAddedConnections(old, new []workflow.Connection) int {
	oldMap := make(map[string]bool)
	for _, conn := range old {
		oldMap[conn.ID] = true
	}

	count := 0
	for _, conn := range new {
		if !oldMap[conn.ID] {
			count++
		}
	}
	return count
}

func countRemovedConnections(old, new []workflow.Connection) int {
	newMap := make(map[string]bool)
	for _, conn := range new {
		newMap[conn.ID] = true
	}

	count := 0
	for _, conn := range old {
		if !newMap[conn.ID] {
			count++
		}
	}
	return count
}
