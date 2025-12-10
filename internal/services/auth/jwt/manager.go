package jwt

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/linkflow-go/pkg/config"
)

type Manager struct {
	privateKey    *rsa.PrivateKey
	publicKey     *rsa.PublicKey
	secretKey     []byte
	issuer        string
	expiry        time.Duration
	refreshExpiry time.Duration
	algorithm     string
}

type Claims struct {
	jwt.RegisteredClaims
	UserID      string   `json:"userId"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
}

type RefreshClaims struct {
	jwt.RegisteredClaims
	UserID string `json:"userId"`
}

func NewManager(cfg config.AuthConfig) (*Manager, error) {
	m := &Manager{
		issuer:        cfg.JWT.Issuer,
		expiry:        time.Duration(cfg.JWT.ExpiryHours) * time.Hour,
		refreshExpiry: time.Duration(cfg.JWT.RefreshDays) * 24 * time.Hour,
		algorithm:     cfg.JWT.Algorithm,
	}
	
	// For RS256 (production), load RSA keys
	if cfg.JWT.Algorithm == "RS256" {
		if cfg.PrivateKeyPath != "" {
			privateKey, err := loadPrivateKey(cfg.PrivateKeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load private key: %w", err)
			}
			m.privateKey = privateKey
		}
		
		if cfg.PublicKeyPath != "" {
			publicKey, err := loadPublicKey(cfg.PublicKeyPath)
			if err != nil {
				return nil, fmt.Errorf("failed to load public key: %w", err)
			}
			m.publicKey = publicKey
		}
		
		if m.privateKey == nil || m.publicKey == nil {
			return nil, errors.New("RS256 algorithm requires both private and public keys")
		}
	} else {
		// For HS256 (development), use secret key
		if cfg.JWT.SecretKey == "" {
			return nil, errors.New("HS256 algorithm requires a secret key")
		}
		m.secretKey = []byte(cfg.JWT.SecretKey)
	}
	
	return m, nil
}

func (m *Manager) GenerateToken(userID, email string, roles, permissions []string) (string, error) {
	claims := Claims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.expiry)),
			ID:        uuid.New().String(),
		},
		UserID:      userID,
		Email:       email,
		Roles:       roles,
		Permissions: permissions,
	}
	
	var token *jwt.Token
	if m.algorithm == "RS256" {
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		return token.SignedString(m.privateKey)
	}
	
	// Default to HS256
	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

func (m *Manager) GenerateRefreshToken(userID string) (string, error) {
	claims := RefreshClaims{
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    m.issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(m.refreshExpiry)),
			ID:        uuid.New().String(),
		},
		UserID: userID,
	}
	
	var token *jwt.Token
	if m.algorithm == "RS256" {
		token = jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		return token.SignedString(m.privateKey)
	}
	
	// Default to HS256
	token = jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(m.secretKey)
}

func (m *Manager) ValidateToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method based on configured algorithm
		if m.algorithm == "RS256" {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return m.publicKey, nil
		}
		
		// Default to HS256
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secretKey, nil
	})
	
	if err != nil {
		return nil, err
	}
	
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	
	return claims, nil
}

func (m *Manager) ValidateRefreshToken(tokenString string) (string, error) {
	token, err := jwt.ParseWithClaims(tokenString, &RefreshClaims{}, func(token *jwt.Token) (interface{}, error) {
		// Validate signing method based on configured algorithm
		if m.algorithm == "RS256" {
			if _, ok := token.Method.(*jwt.SigningMethodRSA); !ok {
				return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
			}
			return m.publicKey, nil
		}
		
		// Default to HS256
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return m.secretKey, nil
	})
	
	if err != nil {
		return "", err
	}
	
	claims, ok := token.Claims.(*RefreshClaims)
	if !ok || !token.Valid {
		return "", errors.New("invalid refresh token")
	}
	
	return claims.UserID, nil
}

// RefreshToken generates a new access token using a valid refresh token
func (m *Manager) RefreshToken(oldToken string) (string, error) {
	// First validate the old access token (even if expired, we just need the claims)
	token, _ := jwt.ParseWithClaims(oldToken, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if m.algorithm == "RS256" {
			return m.publicKey, nil
		}
		return m.secretKey, nil
	})
	
	// Even if the token is expired, we can still extract claims
	if token == nil {
		return "", errors.New("invalid token format")
	}
	
	claims, ok := token.Claims.(*Claims)
	if !ok {
		return "", errors.New("invalid token claims")
	}
	
	// Generate new token with same claims but new expiry
	return m.GenerateToken(claims.UserID, claims.Email, claims.Roles, claims.Permissions)
}

// LoadPrivateKey loads RSA private key from file (for production)
func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	keyData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	
	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	
	return key, nil
}

// LoadPublicKey loads RSA public key from file (for production)
func loadPublicKey(path string) (*rsa.PublicKey, error) {
	keyData, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}
	
	block, _ := pem.Decode(keyData)
	if block == nil {
		return nil, errors.New("failed to decode PEM block")
	}
	
	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	
	key, ok := pub.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("not an RSA public key")
	}
	
	return key, nil
}
