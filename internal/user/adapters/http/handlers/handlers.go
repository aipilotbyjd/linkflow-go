package handlers

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/user/app/service"
	"github.com/linkflow-go/pkg/logger"
)

type UserHandlers struct {
	service *service.UserService
	logger  logger.Logger
}

func NewUserHandlers(service *service.UserService, logger logger.Logger) *UserHandlers {
	return &UserHandlers{
		service: service,
		logger:  logger,
	}
}

// Health check
func (h *UserHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy", "service": "user-service"})
}

func (h *UserHandlers) Ready(c *gin.Context) {
	if err := h.service.CheckReady(); err != nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"status": "not ready", "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "ready", "service": "user-service"})
}

// ========== User Handlers ==========

func (h *UserHandlers) ListUsers(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))
	status := c.Query("status")
	roleID := c.Query("roleId")
	sortBy := c.DefaultQuery("sortBy", "created_at")
	sortDesc := c.Query("sortDesc") == "true"

	resp, err := h.service.ListUsers(c.Request.Context(), service.ListUsersRequest{
		Page:     page,
		Limit:    limit,
		Status:   status,
		RoleID:   roleID,
		SortBy:   sortBy,
		SortDesc: sortDesc,
	})
	if err != nil {
		h.logger.Error("Failed to list users", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list users"})
		return
	}

	c.JSON(http.StatusOK, resp)
}

func (h *UserHandlers) GetUser(c *gin.Context) {
	id := c.Param("id")

	user, err := h.service.GetUser(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user})
}

func (h *UserHandlers) UpdateUser(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateUserRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	user, err := h.service.UpdateUser(c.Request.Context(), id, req)
	if err != nil {
		if err.Error() == "user not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
			return
		}
		h.logger.Error("Failed to update user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"user": user, "message": "User updated successfully"})
}

func (h *UserHandlers) DeleteUser(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteUser(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete user", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete user"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "User deleted successfully"})
}

func (h *UserHandlers) GetUserPermissions(c *gin.Context) {
	id := c.Param("id")

	permissions, err := h.service.GetUserPermissions(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"userId": id, "permissions": permissions})
}

// ========== Team Handlers ==========

func (h *UserHandlers) CreateTeam(c *gin.Context) {
	var req service.CreateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	// Get owner from context (authenticated user)
	if req.OwnerID == "" {
		req.OwnerID = c.GetString("userId")
	}

	team, err := h.service.CreateTeam(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to create team", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create team"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"team": team, "message": "Team created successfully"})
}

func (h *UserHandlers) ListTeams(c *gin.Context) {
	teams, err := h.service.ListTeams(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list teams", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list teams"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"teams": teams})
}

func (h *UserHandlers) GetTeam(c *gin.Context) {
	id := c.Param("id")

	team, err := h.service.GetTeam(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
		return
	}

	// Get members
	members, _ := h.service.GetTeamMembers(c.Request.Context(), id)

	c.JSON(http.StatusOK, gin.H{"team": team, "members": members})
}

func (h *UserHandlers) UpdateTeam(c *gin.Context) {
	id := c.Param("id")

	var req service.UpdateTeamRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request", "details": err.Error()})
		return
	}

	team, err := h.service.UpdateTeam(c.Request.Context(), id, req)
	if err != nil {
		if err.Error() == "team not found" {
			c.JSON(http.StatusNotFound, gin.H{"error": "Team not found"})
			return
		}
		h.logger.Error("Failed to update team", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update team"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"team": team, "message": "Team updated successfully"})
}

func (h *UserHandlers) DeleteTeam(c *gin.Context) {
	id := c.Param("id")

	if err := h.service.DeleteTeam(c.Request.Context(), id); err != nil {
		h.logger.Error("Failed to delete team", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete team"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Team deleted successfully"})
}

func (h *UserHandlers) AddTeamMember(c *gin.Context) {
	teamID := c.Param("id")

	var req struct {
		UserID string `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"})
		return
	}

	if err := h.service.AddTeamMember(c.Request.Context(), teamID, req.UserID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member added successfully"})
}

func (h *UserHandlers) RemoveTeamMember(c *gin.Context) {
	teamID := c.Param("id")
	userID := c.Param("userId")

	if err := h.service.RemoveTeamMember(c.Request.Context(), teamID, userID); err != nil {
		h.logger.Error("Failed to remove team member", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to remove member"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Member removed successfully"})
}

// ========== Role Handlers ==========

func (h *UserHandlers) ListRoles(c *gin.Context) {
	roles, err := h.service.ListRoles(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list roles", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list roles"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"roles": roles})
}

func (h *UserHandlers) CreateRole(c *gin.Context) {
	// For now, roles are predefined
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Custom role creation not supported. Use predefined roles."})
}

func (h *UserHandlers) UpdateRole(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Role modification not supported. Use predefined roles."})
}

func (h *UserHandlers) DeleteRole(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Role deletion not supported. Use predefined roles."})
}

func (h *UserHandlers) AssignRole(c *gin.Context) {
	roleID := c.Param("id")

	var req struct {
		UserID string `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"})
		return
	}

	if err := h.service.AssignRole(c.Request.Context(), req.UserID, roleID); err != nil {
		h.logger.Error("Failed to assign role", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to assign role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role assigned successfully", "userId": req.UserID, "roleId": roleID})
}

func (h *UserHandlers) RevokeRole(c *gin.Context) {
	roleID := c.Param("id")

	var req struct {
		UserID string `json:"userId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "userId is required"})
		return
	}

	if err := h.service.RevokeRole(c.Request.Context(), req.UserID, roleID); err != nil {
		h.logger.Error("Failed to revoke role", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to revoke role"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Role revoked successfully", "userId": req.UserID, "roleId": roleID})
}

// ========== Permission Handlers ==========

func (h *UserHandlers) ListPermissions(c *gin.Context) {
	permissions, err := h.service.ListPermissions(c.Request.Context())
	if err != nil {
		h.logger.Error("Failed to list permissions", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list permissions"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"permissions": permissions})
}

func (h *UserHandlers) CreatePermission(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Custom permission creation not supported"})
}

func (h *UserHandlers) DeletePermission(c *gin.Context) {
	c.JSON(http.StatusNotImplemented, gin.H{"error": "Permission deletion not supported"})
}
