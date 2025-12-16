package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/linkflow-go/internal/user/ports"
	"github.com/linkflow-go/pkg/contracts/user"
	"github.com/linkflow-go/pkg/events"
	"github.com/linkflow-go/pkg/logger"
	"github.com/redis/go-redis/v9"
)

type UserService struct {
	repo     ports.UserRepository
	eventBus events.EventBus
	redis    *redis.Client
	logger   logger.Logger
}

func NewUserService(
	repo ports.UserRepository,
	eventBus events.EventBus,
	redis *redis.Client,
	logger logger.Logger,
) *UserService {
	return &UserService{
		repo:     repo,
		eventBus: eventBus,
		redis:    redis,
		logger:   logger,
	}
}

// ========== User Operations ==========

// GetUser retrieves a user by ID
func (s *UserService) GetUser(ctx context.Context, id string) (*user.User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("user not found: %w", err)
	}
	return u, nil
}

// ListUsers lists users with pagination and filters
func (s *UserService) ListUsers(ctx context.Context, opts ListUsersRequest) (*ListUsersResponse, error) {
	repoOpts := ports.ListUsersOptions{
		Page:         opts.Page,
		Limit:        opts.Limit,
		Status:       opts.Status,
		RoleID:       opts.RoleID,
		SortBy:       opts.SortBy,
		SortDesc:     opts.SortDesc,
		IncludeRoles: true,
	}

	if repoOpts.Page < 1 {
		repoOpts.Page = 1
	}
	if repoOpts.Limit < 1 || repoOpts.Limit > 100 {
		repoOpts.Limit = 20
	}

	users, total, err := s.repo.ListUsers(ctx, repoOpts)
	if err != nil {
		return nil, fmt.Errorf("failed to list users: %w", err)
	}

	return &ListUsersResponse{
		Users: users,
		Total: total,
		Page:  repoOpts.Page,
		Limit: repoOpts.Limit,
	}, nil
}

// UpdateUser updates a user's profile
func (s *UserService) UpdateUser(ctx context.Context, id string, req UpdateUserRequest) (*user.User, error) {
	u, err := s.repo.GetByID(ctx, id)
	if err != nil {
		return nil, errors.New("user not found")
	}

	// Update fields if provided
	if req.FirstName != nil {
		u.FirstName = *req.FirstName
	}
	if req.LastName != nil {
		u.LastName = *req.LastName
	}
	if req.Avatar != nil {
		u.Avatar = *req.Avatar
	}
	if req.Status != nil {
		u.Status = *req.Status
	}

	u.UpdatedAt = time.Now()

	if err := s.repo.Update(ctx, u); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Publish event
	event := events.NewEventBuilder("user.updated").
		WithAggregateID(u.ID).
		WithAggregateType("user").
		WithPayload("userId", u.ID).
		Build()
	s.eventBus.Publish(ctx, event)

	return u, nil
}

// DeleteUser soft deletes a user
func (s *UserService) DeleteUser(ctx context.Context, id string) error {
	if err := s.repo.Delete(ctx, id); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Publish event
	event := events.NewEventBuilder("user.deleted").
		WithAggregateID(id).
		WithAggregateType("user").
		Build()
	s.eventBus.Publish(ctx, event)

	return nil
}

// SearchUsers searches users by query
func (s *UserService) SearchUsers(ctx context.Context, query string, limit int) ([]*user.User, error) {
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	return s.repo.Search(ctx, query, limit)
}

// GetUserPermissions returns user's permissions based on roles
func (s *UserService) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	u, err := s.repo.GetByID(ctx, userID)
	if err != nil {
		return nil, errors.New("user not found")
	}
	return u.GetPermissions(), nil
}

// ========== Team Operations ==========

// CreateTeam creates a new team
func (s *UserService) CreateTeam(ctx context.Context, req CreateTeamRequest) (*Team, error) {
	team := &Team{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     req.OwnerID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// For now, store in Redis as JSON (could be a separate table)
	key := fmt.Sprintf("team:%s", team.ID)
	if err := s.redis.HSet(ctx, key,
		"id", team.ID,
		"name", team.Name,
		"description", team.Description,
		"owner_id", team.OwnerID,
		"created_at", team.CreatedAt.Format(time.RFC3339),
	).Err(); err != nil {
		return nil, fmt.Errorf("failed to create team: %w", err)
	}

	// Add to teams index
	s.redis.SAdd(ctx, "teams:index", team.ID)

	// Add owner as member
	s.redis.SAdd(ctx, fmt.Sprintf("team:%s:members", team.ID), team.OwnerID)

	return team, nil
}

// GetTeam retrieves a team by ID
func (s *UserService) GetTeam(ctx context.Context, id string) (*Team, error) {
	key := fmt.Sprintf("team:%s", id)
	data, err := s.redis.HGetAll(ctx, key).Result()
	if err != nil || len(data) == 0 {
		return nil, errors.New("team not found")
	}

	createdAt, _ := time.Parse(time.RFC3339, data["created_at"])

	return &Team{
		ID:          data["id"],
		Name:        data["name"],
		Description: data["description"],
		OwnerID:     data["owner_id"],
		CreatedAt:   createdAt,
	}, nil
}

// ListTeams lists all teams
func (s *UserService) ListTeams(ctx context.Context) ([]*Team, error) {
	teamIDs, err := s.redis.SMembers(ctx, "teams:index").Result()
	if err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}

	teams := make([]*Team, 0, len(teamIDs))
	for _, id := range teamIDs {
		team, err := s.GetTeam(ctx, id)
		if err == nil {
			teams = append(teams, team)
		}
	}
	return teams, nil
}

// UpdateTeam updates a team
func (s *UserService) UpdateTeam(ctx context.Context, id string, req UpdateTeamRequest) (*Team, error) {
	team, err := s.GetTeam(ctx, id)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		team.Name = *req.Name
	}
	if req.Description != nil {
		team.Description = *req.Description
	}
	team.UpdatedAt = time.Now()

	key := fmt.Sprintf("team:%s", id)
	s.redis.HSet(ctx, key, "name", team.Name, "description", team.Description)

	return team, nil
}

// DeleteTeam deletes a team
func (s *UserService) DeleteTeam(ctx context.Context, id string) error {
	key := fmt.Sprintf("team:%s", id)
	s.redis.Del(ctx, key)
	s.redis.Del(ctx, fmt.Sprintf("team:%s:members", id))
	s.redis.SRem(ctx, "teams:index", id)
	return nil
}

// AddTeamMember adds a user to a team
func (s *UserService) AddTeamMember(ctx context.Context, teamID, userID string) error {
	// Verify team exists
	if _, err := s.GetTeam(ctx, teamID); err != nil {
		return err
	}
	// Verify user exists
	if _, err := s.repo.GetByID(ctx, userID); err != nil {
		return errors.New("user not found")
	}

	s.redis.SAdd(ctx, fmt.Sprintf("team:%s:members", teamID), userID)
	return nil
}

// RemoveTeamMember removes a user from a team
func (s *UserService) RemoveTeamMember(ctx context.Context, teamID, userID string) error {
	s.redis.SRem(ctx, fmt.Sprintf("team:%s:members", teamID), userID)
	return nil
}

// GetTeamMembers gets all members of a team
func (s *UserService) GetTeamMembers(ctx context.Context, teamID string) ([]*user.User, error) {
	memberIDs, err := s.redis.SMembers(ctx, fmt.Sprintf("team:%s:members", teamID)).Result()
	if err != nil {
		return nil, err
	}

	users := make([]*user.User, 0, len(memberIDs))
	for _, id := range memberIDs {
		u, err := s.repo.GetByID(ctx, id)
		if err == nil {
			users = append(users, u)
		}
	}
	return users, nil
}

// ========== Role Operations ==========

// ListRoles returns all available roles
func (s *UserService) ListRoles(ctx context.Context) ([]*Role, error) {
	// Return predefined roles
	return []*Role{
		{ID: "admin", Name: "Admin", Description: "Full access to all resources", Permissions: []string{"*"}},
		{ID: "user", Name: "User", Description: "Standard user access", Permissions: []string{"workflows:read", "workflows:write", "executions:read"}},
		{ID: "viewer", Name: "Viewer", Description: "Read-only access", Permissions: []string{"workflows:read", "executions:read"}},
		{ID: "developer", Name: "Developer", Description: "Developer access", Permissions: []string{"workflows:*", "executions:*", "credentials:read"}},
	}, nil
}

// AssignRole assigns a role to a user
func (s *UserService) AssignRole(ctx context.Context, userID, roleID string) error {
	return s.repo.AssignRole(ctx, userID, roleID)
}

// RevokeRole removes a role from a user
func (s *UserService) RevokeRole(ctx context.Context, userID, roleID string) error {
	return s.repo.RemoveRole(ctx, userID, roleID)
}

// ========== Permission Operations ==========

// ListPermissions returns all available permissions
func (s *UserService) ListPermissions(ctx context.Context) ([]string, error) {
	return []string{
		"workflows:read", "workflows:write", "workflows:delete", "workflows:execute",
		"executions:read", "executions:write", "executions:delete",
		"credentials:read", "credentials:write", "credentials:delete",
		"users:read", "users:write", "users:delete",
		"teams:read", "teams:write", "teams:delete",
		"admin:*",
	}, nil
}

// ========== Event Handlers ==========

func (s *UserService) HandleUserRegistered(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling user registered event", "type", event.Type, "id", event.ID)
	return nil
}

func (s *UserService) HandleUserDeleted(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling user deleted event", "type", event.Type, "id", event.ID)
	return nil
}

func (s *UserService) HandleWorkflowCreated(ctx context.Context, event events.Event) error {
	s.logger.Info("Handling workflow created event for ownership tracking")
	return nil
}

func (s *UserService) CheckReady() error {
	return nil
}

// ========== Request/Response Types ==========

type ListUsersRequest struct {
	Page     int
	Limit    int
	Status   string
	RoleID   string
	SortBy   string
	SortDesc bool
}

type ListUsersResponse struct {
	Users []*user.User `json:"users"`
	Total int64        `json:"total"`
	Page  int          `json:"page"`
	Limit int          `json:"limit"`
}

type UpdateUserRequest struct {
	FirstName *string `json:"firstName,omitempty"`
	LastName  *string `json:"lastName,omitempty"`
	Avatar    *string `json:"avatar,omitempty"`
	Status    *string `json:"status,omitempty"`
}

type CreateTeamRequest struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	OwnerID     string `json:"ownerId"`
}

type UpdateTeamRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

type Team struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	OwnerID     string    `json:"ownerId"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type Role struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Permissions []string `json:"permissions"`
}
