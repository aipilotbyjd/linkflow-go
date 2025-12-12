package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/linkflow-go/internal/domain/user"
	"github.com/linkflow-go/pkg/cache"
	"github.com/linkflow-go/pkg/database"
)

// CachedUserRepository wraps UserRepository with caching
type CachedUserRepository struct {
	repo  *UserRepository
	cache cache.Cache
	ttl   time.Duration
}

// NewCachedUserRepository creates a new cached user repository
func NewCachedUserRepository(db *database.DB, c cache.Cache) *CachedUserRepository {
	return &CachedUserRepository{
		repo:  NewUserRepository(db),
		cache: c,
		ttl:   5 * time.Minute,
	}
}

// GetByID retrieves a user by ID with caching
func (r *CachedUserRepository) GetByID(ctx context.Context, id string) (*user.User, error) {
	cacheKey := fmt.Sprintf("id:%s", id)

	var u user.User

	// Try cache first
	err := r.cache.Get(ctx, cacheKey, &u)
	if err == nil {
		return &u, nil
	}

	// Cache miss - get from database
	dbUser, err := r.repo.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}

	// Store in cache
	_ = r.cache.Set(ctx, cacheKey, dbUser, r.ttl)

	return dbUser, nil
}

// GetByEmail retrieves a user by email with caching
func (r *CachedUserRepository) GetByEmail(ctx context.Context, email string) (*user.User, error) {
	cacheKey := fmt.Sprintf("email:%s", email)

	var u user.User

	// Try cache first
	err := r.cache.Get(ctx, cacheKey, &u)
	if err == nil {
		return &u, nil
	}

	// Cache miss - get from database
	dbUser, err := r.repo.GetByEmail(ctx, email)
	if err != nil {
		return nil, err
	}

	// Store in cache
	_ = r.cache.Set(ctx, cacheKey, dbUser, r.ttl)

	// Also cache by ID for future lookups
	_ = r.cache.Set(ctx, fmt.Sprintf("id:%s", dbUser.ID), dbUser, r.ttl)

	return dbUser, nil
}

// Create creates a new user and invalidates relevant caches
func (r *CachedUserRepository) Create(ctx context.Context, u *user.User) error {
	err := r.repo.Create(ctx, u)
	if err != nil {
		return err
	}

	// Cache the new user
	_ = r.cache.Set(ctx, fmt.Sprintf("id:%s", u.ID), u, r.ttl)
	_ = r.cache.Set(ctx, fmt.Sprintf("email:%s", u.Email), u, r.ttl)

	// Invalidate list caches
	_ = r.cache.Invalidate(ctx, "list:*")
	_ = r.cache.Invalidate(ctx, "count:*")

	return nil
}

// Update updates a user and invalidates caches
func (r *CachedUserRepository) Update(ctx context.Context, u *user.User) error {
	// Get old user data for cache invalidation
	oldUser, _ := r.repo.GetByID(ctx, u.ID)

	err := r.repo.Update(ctx, u)
	if err != nil {
		return err
	}

	// Invalidate old caches
	if oldUser != nil && oldUser.Email != u.Email {
		_ = r.cache.Delete(ctx, fmt.Sprintf("email:%s", oldUser.Email))
	}

	// Update caches
	_ = r.cache.Set(ctx, fmt.Sprintf("id:%s", u.ID), u, r.ttl)
	_ = r.cache.Set(ctx, fmt.Sprintf("email:%s", u.Email), u, r.ttl)
	if u.Username != "" {
		_ = r.cache.Set(ctx, fmt.Sprintf("username:%s", u.Username), u, r.ttl)
	}

	// Invalidate list caches
	_ = r.cache.Invalidate(ctx, "list:*")

	return nil
}

// Delete soft deletes a user and invalidates caches
func (r *CachedUserRepository) Delete(ctx context.Context, id string) error {
	// Get user for cache invalidation
	u, _ := r.repo.GetByID(ctx, id)

	err := r.repo.Delete(ctx, id)
	if err != nil {
		return err
	}

	// Invalidate caches
	_ = r.cache.Delete(ctx, fmt.Sprintf("id:%s", id))
	if u != nil {
		_ = r.cache.Delete(ctx, fmt.Sprintf("email:%s", u.Email))
		if u.Username != "" {
			_ = r.cache.Delete(ctx, fmt.Sprintf("username:%s", u.Username))
		}
	}

	// Invalidate list caches
	_ = r.cache.Invalidate(ctx, "list:*")
	_ = r.cache.Invalidate(ctx, "count:*")

	return nil
}

// ListUsers lists users with caching
func (r *CachedUserRepository) ListUsers(ctx context.Context, opts ListUsersOptions) ([]*user.User, int64, error) {
	// Create cache key based on options
	cacheKey := fmt.Sprintf("list:%d:%d:%s:%s:%s:%v:%v",
		opts.Page, opts.Limit, opts.Status, opts.RoleID,
		opts.TeamID, opts.SortBy, opts.SortDesc)

	type listResult struct {
		Users []*user.User
		Total int64
	}

	var result listResult

	// Try cache first
	err := r.cache.Get(ctx, cacheKey, &result)
	if err == nil {
		return result.Users, result.Total, nil
	}

	// Cache miss - get from database
	users, total, err := r.repo.ListUsers(ctx, opts)
	if err != nil {
		return nil, 0, err
	}

	// Store in cache with shorter TTL for list operations
	result = listResult{Users: users, Total: total}
	_ = r.cache.Set(ctx, cacheKey, result, 1*time.Minute)

	// Cache individual users
	for _, u := range users {
		_ = r.cache.Set(ctx, fmt.Sprintf("id:%s", u.ID), u, r.ttl)
	}

	return users, total, nil
}

// GetActiveUsersCount returns cached count of active users
func (r *CachedUserRepository) GetActiveUsersCount(ctx context.Context) (int64, error) {
	cacheKey := "count:active"

	var count int64

	// Try cache first
	err := r.cache.Get(ctx, cacheKey, &count)
	if err == nil {
		return count, nil
	}

	// Cache miss - get from database
	count, err = r.repo.GetActiveUsersCount(ctx)
	if err != nil {
		return 0, err
	}

	// Store in cache with shorter TTL
	_ = r.cache.Set(ctx, cacheKey, count, 30*time.Second)

	return count, nil
}

// Search searches users with caching
func (r *CachedUserRepository) Search(ctx context.Context, query string, limit int) ([]*user.User, error) {
	cacheKey := fmt.Sprintf("search:%s:%d", query, limit)

	var users []*user.User

	// Try cache first
	err := r.cache.Get(ctx, cacheKey, &users)
	if err == nil {
		return users, nil
	}

	// Cache miss - search in database
	users, err = r.repo.Search(ctx, query, limit)
	if err != nil {
		return nil, err
	}

	// Store in cache with shorter TTL for search results
	_ = r.cache.Set(ctx, cacheKey, users, 30*time.Second)

	return users, nil
}

// WarmUpCache pre-loads frequently accessed users into cache
func (r *CachedUserRepository) WarmUpCache(ctx context.Context) error {
	// Get recently active users
	users, _, err := r.repo.ListUsers(ctx, ListUsersOptions{
		Status: user.StatusActive,
		Page:   1,
		Limit:  100,
		SortBy: "last_login_at",
	})

	if err != nil {
		return err
	}

	// Cache users
	for _, u := range users {
		_ = r.cache.Set(ctx, fmt.Sprintf("id:%s", u.ID), u, r.ttl)
		_ = r.cache.Set(ctx, fmt.Sprintf("email:%s", u.Email), u, r.ttl)
	}

	return nil
}

// InvalidateUserCache invalidates all caches for a specific user
func (r *CachedUserRepository) InvalidateUserCache(ctx context.Context, userID string) error {
	u, err := r.repo.GetByID(ctx, userID)
	if err != nil {
		return err
	}

	// Delete specific user caches
	_ = r.cache.Delete(ctx, fmt.Sprintf("id:%s", userID))
	_ = r.cache.Delete(ctx, fmt.Sprintf("email:%s", u.Email))
	if u.Username != "" {
		_ = r.cache.Delete(ctx, fmt.Sprintf("username:%s", u.Username))
	}

	// Invalidate list caches that might contain this user
	_ = r.cache.Invalidate(ctx, "list:*")
	_ = r.cache.Invalidate(ctx, "search:*")

	return nil
}

// Delegate all other methods to the underlying repository
func (r *CachedUserRepository) GetByUsername(ctx context.Context, username string) (*user.User, error) {
	cacheKey := fmt.Sprintf("username:%s", username)

	var u user.User

	// Try cache first
	err := r.cache.Get(ctx, cacheKey, &u)
	if err == nil {
		return &u, nil
	}

	// Cache miss - get from database
	dbUser, err := r.repo.GetByUsername(ctx, username)
	if err != nil {
		return nil, err
	}

	// Store in cache
	_ = r.cache.Set(ctx, cacheKey, dbUser, r.ttl)

	return dbUser, nil
}

func (r *CachedUserRepository) UpdateLastLogin(ctx context.Context, userID string) error {
	err := r.repo.UpdateLastLogin(ctx, userID)
	if err != nil {
		return err
	}

	// Invalidate user cache to force reload with new login time
	_ = r.InvalidateUserCache(ctx, userID)

	return nil
}

func (r *CachedUserRepository) HardDelete(ctx context.Context, id string) error {
	return r.Delete(ctx, id) // Use same cache invalidation logic
}

func (r *CachedUserRepository) BulkUpdate(ctx context.Context, userIDs []string, updates map[string]interface{}) error {
	err := r.repo.BulkUpdate(ctx, userIDs, updates)
	if err != nil {
		return err
	}

	// Invalidate caches for all updated users
	for _, id := range userIDs {
		_ = r.InvalidateUserCache(ctx, id)
	}

	return nil
}

func (r *CachedUserRepository) BulkDelete(ctx context.Context, userIDs []string) error {
	err := r.repo.BulkDelete(ctx, userIDs)
	if err != nil {
		return err
	}

	// Invalidate caches for all deleted users
	for _, id := range userIDs {
		_ = r.InvalidateUserCache(ctx, id)
	}

	return nil
}

func (r *CachedUserRepository) AssignRole(ctx context.Context, userID, roleID string) error {
	err := r.repo.AssignRole(ctx, userID, roleID)
	if err != nil {
		return err
	}

	// Invalidate user cache as roles have changed
	_ = r.InvalidateUserCache(ctx, userID)

	return nil
}

func (r *CachedUserRepository) RemoveRole(ctx context.Context, userID, roleID string) error {
	err := r.repo.RemoveRole(ctx, userID, roleID)
	if err != nil {
		return err
	}

	// Invalidate user cache as roles have changed
	_ = r.InvalidateUserCache(ctx, userID)

	return nil
}

func (r *CachedUserRepository) GetUsersByRole(ctx context.Context, roleID string) ([]*user.User, error) {
	cacheKey := fmt.Sprintf("role:%s", roleID)

	var users []*user.User

	// Try cache first
	err := r.cache.Get(ctx, cacheKey, &users)
	if err == nil {
		return users, nil
	}

	// Cache miss - get from database
	users, err = r.repo.GetUsersByRole(ctx, roleID)
	if err != nil {
		return nil, err
	}

	// Store in cache
	_ = r.cache.Set(ctx, cacheKey, users, 1*time.Minute)

	return users, nil
}

func (r *CachedUserRepository) GetUsersByTeam(ctx context.Context, teamID string) ([]*user.User, error) {
	cacheKey := fmt.Sprintf("team:%s", teamID)

	var users []*user.User

	// Try cache first
	err := r.cache.Get(ctx, cacheKey, &users)
	if err == nil {
		return users, nil
	}

	// Cache miss - get from database
	users, err = r.repo.GetUsersByTeam(ctx, teamID)
	if err != nil {
		return nil, err
	}

	// Store in cache
	_ = r.cache.Set(ctx, cacheKey, users, 1*time.Minute)

	return users, nil
}

func (r *CachedUserRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	// Don't cache existence checks to avoid stale data issues
	return r.repo.ExistsByEmail(ctx, email)
}

func (r *CachedUserRepository) ExistsByUsername(ctx context.Context, username string) (bool, error) {
	// Don't cache existence checks to avoid stale data issues
	return r.repo.ExistsByUsername(ctx, username)
}
