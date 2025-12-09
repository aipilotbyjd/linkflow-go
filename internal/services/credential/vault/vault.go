package vault

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"

	"github.com/linkflow-go/internal/domain/credential"
	"github.com/linkflow-go/pkg/logger"
)

type VaultManager struct {
	encryptionKey []byte
	logger        logger.Logger
}

func NewVaultManager(key string, logger logger.Logger) (*VaultManager, error) {
	if len(key) != 32 {
		return nil, errors.New("encryption key must be 32 bytes")
	}
	
	return &VaultManager{
		encryptionKey: []byte(key),
		logger:        logger,
	}, nil
}

// Encrypt encrypts credential data
func (v *VaultManager) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(v.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	// Create GCM mode
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	// Create nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt data
	ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	
	// Encode to base64
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decrypts credential data
func (v *VaultManager) Decrypt(ciphertext string) (string, error) {
	// Decode from base64
	data, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("failed to decode ciphertext: %w", err)
	}

	block, err := aes.NewCipher(v.encryptionKey)
	if err != nil {
		return "", fmt.Errorf("failed to create cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("failed to create GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", errors.New("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	
	// Decrypt data
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", fmt.Errorf("failed to decrypt: %w", err)
	}

	return string(plaintext), nil
}

// EncryptCredential encrypts a credential's sensitive data
func (v *VaultManager) EncryptCredential(ctx context.Context, cred *credential.Credential) error {
	// Encrypt based on credential type
	switch cred.Type {
	case credential.TypeAPIKey:
		if apiKey, ok := cred.Data["apiKey"].(string); ok {
			encrypted, err := v.Encrypt(apiKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt API key: %w", err)
			}
			cred.Data["apiKey"] = encrypted
			cred.Data["encrypted"] = true
		}
		
	case credential.TypeOAuth2:
		// Encrypt access and refresh tokens
		if accessToken, ok := cred.Data["accessToken"].(string); ok {
			encrypted, err := v.Encrypt(accessToken)
			if err != nil {
				return fmt.Errorf("failed to encrypt access token: %w", err)
			}
			cred.Data["accessToken"] = encrypted
		}
		
		if refreshToken, ok := cred.Data["refreshToken"].(string); ok {
			encrypted, err := v.Encrypt(refreshToken)
			if err != nil {
				return fmt.Errorf("failed to encrypt refresh token: %w", err)
			}
			cred.Data["refreshToken"] = encrypted
		}
		
		if clientSecret, ok := cred.Data["clientSecret"].(string); ok {
			encrypted, err := v.Encrypt(clientSecret)
			if err != nil {
				return fmt.Errorf("failed to encrypt client secret: %w", err)
			}
			cred.Data["clientSecret"] = encrypted
		}
		cred.Data["encrypted"] = true
		
	case credential.TypeBasicAuth:
		if password, ok := cred.Data["password"].(string); ok {
			encrypted, err := v.Encrypt(password)
			if err != nil {
				return fmt.Errorf("failed to encrypt password: %w", err)
			}
			cred.Data["password"] = encrypted
			cred.Data["encrypted"] = true
		}
		
	case credential.TypeSSHKey:
		if privateKey, ok := cred.Data["privateKey"].(string); ok {
			encrypted, err := v.Encrypt(privateKey)
			if err != nil {
				return fmt.Errorf("failed to encrypt private key: %w", err)
			}
			cred.Data["privateKey"] = encrypted
		}
		
		if passphrase, ok := cred.Data["passphrase"].(string); ok && passphrase != "" {
			encrypted, err := v.Encrypt(passphrase)
			if err != nil {
				return fmt.Errorf("failed to encrypt passphrase: %w", err)
			}
			cred.Data["passphrase"] = encrypted
		}
		cred.Data["encrypted"] = true
		
	case credential.TypeDatabase:
		if password, ok := cred.Data["password"].(string); ok {
			encrypted, err := v.Encrypt(password)
			if err != nil {
				return fmt.Errorf("failed to encrypt database password: %w", err)
			}
			cred.Data["password"] = encrypted
		}
		
		if connectionString, ok := cred.Data["connectionString"].(string); ok {
			encrypted, err := v.Encrypt(connectionString)
			if err != nil {
				return fmt.Errorf("failed to encrypt connection string: %w", err)
			}
			cred.Data["connectionString"] = encrypted
		}
		cred.Data["encrypted"] = true
	}
	
	return nil
}

// DecryptCredential decrypts a credential's sensitive data
func (v *VaultManager) DecryptCredential(ctx context.Context, cred *credential.Credential) error {
	// Check if already decrypted
	if encrypted, ok := cred.Data["encrypted"].(bool); !ok || !encrypted {
		return nil
	}
	
	// Decrypt based on credential type
	switch cred.Type {
	case credential.TypeAPIKey:
		if apiKey, ok := cred.Data["apiKey"].(string); ok {
			decrypted, err := v.Decrypt(apiKey)
			if err != nil {
				return fmt.Errorf("failed to decrypt API key: %w", err)
			}
			cred.Data["apiKey"] = decrypted
		}
		
	case credential.TypeOAuth2:
		if accessToken, ok := cred.Data["accessToken"].(string); ok {
			decrypted, err := v.Decrypt(accessToken)
			if err != nil {
				return fmt.Errorf("failed to decrypt access token: %w", err)
			}
			cred.Data["accessToken"] = decrypted
		}
		
		if refreshToken, ok := cred.Data["refreshToken"].(string); ok {
			decrypted, err := v.Decrypt(refreshToken)
			if err != nil {
				return fmt.Errorf("failed to decrypt refresh token: %w", err)
			}
			cred.Data["refreshToken"] = decrypted
		}
		
		if clientSecret, ok := cred.Data["clientSecret"].(string); ok {
			decrypted, err := v.Decrypt(clientSecret)
			if err != nil {
				return fmt.Errorf("failed to decrypt client secret: %w", err)
			}
			cred.Data["clientSecret"] = decrypted
		}
		
	case credential.TypeBasicAuth:
		if password, ok := cred.Data["password"].(string); ok {
			decrypted, err := v.Decrypt(password)
			if err != nil {
				return fmt.Errorf("failed to decrypt password: %w", err)
			}
			cred.Data["password"] = decrypted
		}
		
	case credential.TypeSSHKey:
		if privateKey, ok := cred.Data["privateKey"].(string); ok {
			decrypted, err := v.Decrypt(privateKey)
			if err != nil {
				return fmt.Errorf("failed to decrypt private key: %w", err)
			}
			cred.Data["privateKey"] = decrypted
		}
		
		if passphrase, ok := cred.Data["passphrase"].(string); ok && passphrase != "" {
			decrypted, err := v.Decrypt(passphrase)
			if err != nil {
				return fmt.Errorf("failed to decrypt passphrase: %w", err)
			}
			cred.Data["passphrase"] = decrypted
		}
		
	case credential.TypeDatabase:
		if password, ok := cred.Data["password"].(string); ok {
			decrypted, err := v.Decrypt(password)
			if err != nil {
				return fmt.Errorf("failed to decrypt database password: %w", err)
			}
			cred.Data["password"] = decrypted
		}
		
		if connectionString, ok := cred.Data["connectionString"].(string); ok {
			decrypted, err := v.Decrypt(connectionString)
			if err != nil {
				return fmt.Errorf("failed to decrypt connection string: %w", err)
			}
			cred.Data["connectionString"] = decrypted
		}
	}
	
	// Mark as decrypted
	cred.Data["encrypted"] = false
	
	return nil
}

// RotateEncryptionKey rotates the encryption key
func (v *VaultManager) RotateEncryptionKey(ctx context.Context, newKey string, credentials []*credential.Credential) error {
	if len(newKey) != 32 {
		return errors.New("new encryption key must be 32 bytes")
	}
	
	// Decrypt all credentials with old key
	for _, cred := range credentials {
		if err := v.DecryptCredential(ctx, cred); err != nil {
			return fmt.Errorf("failed to decrypt credential %s: %w", cred.ID, err)
		}
	}
	
	// Update encryption key
	oldKey := v.encryptionKey
	v.encryptionKey = []byte(newKey)
	
	// Re-encrypt all credentials with new key
	for _, cred := range credentials {
		if err := v.EncryptCredential(ctx, cred); err != nil {
			// Rollback on error
			v.encryptionKey = oldKey
			return fmt.Errorf("failed to re-encrypt credential %s: %w", cred.ID, err)
		}
	}
	
	v.logger.Info("Successfully rotated encryption key", "credentials", len(credentials))
	return nil
}

// GenerateEncryptionKey generates a new 32-byte encryption key
func GenerateEncryptionKey() (string, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return "", fmt.Errorf("failed to generate key: %w", err)
	}
	return base64.StdEncoding.EncodeToString(key), nil
}
