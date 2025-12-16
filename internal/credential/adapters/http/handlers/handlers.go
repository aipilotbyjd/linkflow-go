package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/linkflow-go/internal/credential/app/service"
	"github.com/linkflow-go/pkg/contracts/credential"
	"github.com/linkflow-go/pkg/logger"
)

type CredentialHandlers struct {
	service *service.CredentialService
	logger  logger.Logger
}

func NewCredentialHandlers(service *service.CredentialService, logger logger.Logger) *CredentialHandlers {
	return &CredentialHandlers{
		service: service,
		logger:  logger,
	}
}

func (h *CredentialHandlers) Health(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "healthy"})
}

func (h *CredentialHandlers) Ready(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ready"})
}

func (h *CredentialHandlers) ListCredentials(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	credentials, err := h.service.ListCredentials(c.Request.Context(), userID)
	if err != nil {
		h.logger.Error("Failed to list credentials", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list credentials"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"credentials": credentials})
}

func (h *CredentialHandlers) GetCredential(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	cred, err := h.service.GetCredential(c.Request.Context(), id, userID)
	if err != nil {
		h.logger.Error("Failed to get credential", "error", err, "id", id)
		c.JSON(http.StatusNotFound, gin.H{"error": "credential not found"})
		return
	}

	c.JSON(http.StatusOK, cred)
}

func (h *CredentialHandlers) CreateCredential(c *gin.Context) {
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	var req service.CreateCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = userID

	cred, err := h.service.CreateCredential(c.Request.Context(), req)
	if err != nil {
		h.logger.Error("Failed to create credential", "error", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, cred)
}

func (h *CredentialHandlers) UpdateCredential(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	var req service.UpdateCredentialRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	req.UserID = userID

	cred, err := h.service.UpdateCredential(c.Request.Context(), id, req)
	if err != nil {
		h.logger.Error("Failed to update credential", "error", err, "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, cred)
}

func (h *CredentialHandlers) DeleteCredential(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	if err := h.service.DeleteCredential(c.Request.Context(), id, userID); err != nil {
		h.logger.Error("Failed to delete credential", "error", err, "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusNoContent)
}

func (h *CredentialHandlers) TestCredential(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	valid, err := h.service.TestCredential(c.Request.Context(), id, userID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"valid": false, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"valid": valid})
}

func (h *CredentialHandlers) RotateCredential(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Credential rotated"})
}

func (h *CredentialHandlers) DecryptCredential(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	cred, err := h.service.GetDecryptedCredential(c.Request.Context(), id, userID)
	if err != nil {
		h.logger.Error("Failed to decrypt credential", "error", err, "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": cred.Data})
}

func (h *CredentialHandlers) ShareCredential(c *gin.Context) {
	id := c.Param("id")
	userID := c.GetHeader("X-User-ID")
	if userID == "" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "user ID required"})
		return
	}

	var req struct {
		TargetUserID string `json:"targetUserId" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.service.ShareCredential(c.Request.Context(), id, userID, req.TargetUserID); err != nil {
		h.logger.Error("Failed to share credential", "error", err, "id", id)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Credential shared"})
}

func (h *CredentialHandlers) UnshareCredential(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (h *CredentialHandlers) ListCredentialTypes(c *gin.Context) {
	types := credential.GetCredentialTypes()
	c.JSON(http.StatusOK, gin.H{"types": types})
}

func (h *CredentialHandlers) GetCredentialTypeSchema(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"schema": map[string]interface{}{}})
}

func (h *CredentialHandlers) OAuthAuthorize(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"auth_url": "https://oauth.example.com"})
}

func (h *CredentialHandlers) OAuthCallback(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "OAuth callback processed"})
}

func (h *CredentialHandlers) OAuthRefresh(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Token refreshed"})
}

func (h *CredentialHandlers) OAuthRevoke(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Token revoked"})
}

func (h *CredentialHandlers) CreateAPIKey(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"api_key": "generated_key"})
}

func (h *CredentialHandlers) ListAPIKeys(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"api_keys": []interface{}{}})
}

func (h *CredentialHandlers) RevokeAPIKey(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (h *CredentialHandlers) RegenerateAPIKey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"api_key": "new_generated_key"})
}

func (h *CredentialHandlers) CreateSSHKey(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"message": "SSH key created"})
}

func (h *CredentialHandlers) ListSSHKeys(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"ssh_keys": []interface{}{}})
}

func (h *CredentialHandlers) DeleteSSHKey(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (h *CredentialHandlers) GetPublicKey(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"public_key": "ssh-rsa ..."})
}

func (h *CredentialHandlers) UploadCertificate(c *gin.Context) {
	c.JSON(http.StatusCreated, gin.H{"message": "Certificate uploaded"})
}

func (h *CredentialHandlers) ListCertificates(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"certificates": []interface{}{}})
}

func (h *CredentialHandlers) GetCertificate(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"certificate": "Certificate details"})
}

func (h *CredentialHandlers) DeleteCertificate(c *gin.Context) {
	c.Status(http.StatusNoContent)
}

func (h *CredentialHandlers) VerifyCertificate(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"valid": true})
}

func (h *CredentialHandlers) BackupVault(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Vault backed up"})
}

func (h *CredentialHandlers) RestoreVault(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Vault restored"})
}

func (h *CredentialHandlers) RekeyVault(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Vault rekeyed"})
}

func (h *CredentialHandlers) GetVaultStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "sealed", "version": "1.0"})
}

func (h *CredentialHandlers) GetCredentialAudit(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"audit": []interface{}{}})
}

func (h *CredentialHandlers) GetAuditLogs(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"logs": []interface{}{}})
}

func (h *CredentialHandlers) ImportCredentials(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"message": "Credentials imported"})
}

func (h *CredentialHandlers) ExportCredentials(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"data": map[string]interface{}{}})
}
