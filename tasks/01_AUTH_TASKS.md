# 🔐 Authentication & Authorization Tasks

## Prerequisites
- Database migrations must be run (DATA-001)
- Redis must be running (INFRA-001)

---

## AUTH-001: Implement JWT Token Generation
**Priority**: P0 | **Hours**: 4 | **Dependencies**: None

### Context
The auth service needs to generate JWT tokens for authenticated users. This is critical for all API access.

### Implementation
**Files to modify:**
- `internal/services/auth/jwt/manager.go`
- `internal/services/auth/service/service.go`
- `pkg/config/config.go` (add JWT config)

### Steps
1. Add JWT configuration to config:
```go
type JWTConfig struct {
    SecretKey    string `mapstructure:"secret_key"`
    ExpiryHours  int    `mapstructure:"expiry_hours"`
    RefreshDays  int    `mapstructure:"refresh_days"`
    Issuer       string `mapstructure:"issuer"`
}
```

2. Implement JWT manager:
```go
func (m *JWTManager) GenerateToken(userID string, email string, roles []string) (string, error)
func (m *JWTManager) ValidateToken(tokenString string) (*Claims, error)
func (m *JWTManager) RefreshToken(oldToken string) (string, error)
```

3. Add RS256 support for production

### Testing
```bash
# Test token generation
curl -X POST localhost:8081/auth/login -d '{"email":"test@test.com","password":"pass"}'

# Verify token validation
curl -H "Authorization: Bearer <token>" localhost:8081/auth/validate
```

### Acceptance Criteria
- ✅ JWT tokens generated with proper claims
- ✅ Tokens expire after configured time
- ✅ Token validation works
- ✅ Refresh token mechanism works
- ✅ Unit tests pass

---

## AUTH-002: Implement User Login Endpoint
**Priority**: P0 | **Hours**: 3 | **Dependencies**: AUTH-001, DATA-002

### Context
Users need to authenticate with email/password to receive JWT tokens.

### Implementation
**Files to modify:**
- `internal/services/auth/handlers/handlers.go`
- `internal/services/auth/repository/repository.go`
- `internal/domain/user/user.go`

### Steps
1. Implement login handler:
```go
func (h *AuthHandlers) Login(c *gin.Context) {
    var req LoginRequest
    // Validate request
    // Check user credentials
    // Generate JWT
    // Return token
}
```

2. Add password hashing with bcrypt
3. Implement rate limiting for failed attempts
4. Add login event publishing

### Testing
```bash
# Test successful login
curl -X POST localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"SecurePass123"}'

# Test failed login
curl -X POST localhost:8081/auth/login \
  -H "Content-Type: application/json" \
  -d '{"email":"user@example.com","password":"wrong"}'
```

### Acceptance Criteria
- ✅ Successful login returns JWT token
- ✅ Failed login returns 401
- ✅ Password properly hashed
- ✅ Rate limiting prevents brute force
- ✅ Login events published to Kafka

---

## AUTH-003: Implement User Registration
**Priority**: P0 | **Hours**: 4 | **Dependencies**: DATA-002

### Context
New users need to register accounts with email verification.

### Implementation
**Files to modify:**
- `internal/services/auth/handlers/handlers.go`
- `internal/services/user/service/service.go`
- `internal/services/notification/service/service.go`

### Steps
1. Add registration handler
2. Implement email validation
3. Create verification token
4. Send verification email
5. Store user in pending state

### Testing
```bash
# Register new user
curl -X POST localhost:8081/auth/register \
  -d '{"email":"new@example.com","password":"Pass123!","name":"John Doe"}'

# Verify email
curl -X GET "localhost:8081/auth/verify?token=<verification_token>"
```

### Acceptance Criteria
- ✅ User registration creates pending account
- ✅ Verification email sent
- ✅ Email verification activates account
- ✅ Duplicate emails rejected
- ✅ Password complexity enforced

---

## AUTH-004: Implement OAuth2 Provider Integration
**Priority**: P1 | **Hours**: 6 | **Dependencies**: AUTH-001

### Context
Support login via Google, GitHub, Microsoft OAuth2 providers.

### Implementation
**Files to modify:**
- `internal/services/auth/oauth/providers.go` (create)
- `internal/services/auth/handlers/oauth_handlers.go` (create)
- `configs/oauth_providers.yaml` (create)

### Steps
1. Implement OAuth2 provider interface
2. Add Google OAuth2 provider
3. Add GitHub OAuth2 provider
4. Store OAuth tokens securely
5. Link OAuth accounts to users

### Testing
```bash
# Initiate OAuth flow
curl -X GET localhost:8081/auth/oauth/google/login

# Handle callback
curl -X GET "localhost:8081/auth/oauth/google/callback?code=<code>&state=<state>"
```

### Acceptance Criteria
- ✅ OAuth2 flow works for Google
- ✅ OAuth2 flow works for GitHub
- ✅ OAuth accounts linked to users
- ✅ Tokens stored securely
- ✅ State parameter prevents CSRF

---

## AUTH-005: Implement RBAC with Casbin
**Priority**: P1 | **Hours**: 5 | **Dependencies**: AUTH-001, DATA-003

### Context
Role-based access control needed for authorization across all services.

### Implementation
**Files to modify:**
- `internal/services/auth/rbac/casbin.go` (create)
- `configs/rbac_model.conf` (create)
- `configs/rbac_policy.csv` (create)

### Steps
1. Setup Casbin with model
2. Define roles: admin, user, viewer
3. Define permissions per service
4. Implement permission checking
5. Add middleware for authorization

### Example Casbin Model:
```ini
[request_definition]
r = sub, obj, act

[policy_definition]
p = sub, obj, act

[role_definition]
g = _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.sub, p.sub) && r.obj == p.obj && r.act == p.act
```

### Acceptance Criteria
- ✅ Roles properly defined
- ✅ Permissions enforced
- ✅ Admin can access everything
- ✅ Users limited to own resources
- ✅ Permission checks < 5ms

---

## AUTH-006: Implement Two-Factor Authentication
**Priority**: P2 | **Hours**: 4 | **Dependencies**: AUTH-001, AUTH-002

### Context
Add TOTP-based 2FA for enhanced security.

### Implementation
**Files to modify:**
- `internal/services/auth/totp/totp.go` (create)
- `internal/services/auth/handlers/twofa_handlers.go` (create)

### Steps
1. Generate TOTP secrets
2. Generate QR codes
3. Verify TOTP codes
4. Store recovery codes
5. Add 2FA to login flow

### Testing
```bash
# Enable 2FA
curl -X POST localhost:8081/auth/2fa/enable \
  -H "Authorization: Bearer <token>"

# Verify with TOTP
curl -X POST localhost:8081/auth/2fa/verify \
  -d '{"code":"123456"}'
```

### Acceptance Criteria
- ✅ TOTP secrets generated
- ✅ QR codes work with Google Authenticator
- ✅ TOTP verification works
- ✅ Recovery codes generated
- ✅ 2FA required when enabled

---

## AUTH-007: Implement Session Management
**Priority**: P1 | **Hours**: 3 | **Dependencies**: AUTH-001

### Context
Track active user sessions with Redis for single sign-out capability.

### Implementation
**Files to modify:**
- `internal/services/auth/session/manager.go` (create)
- `internal/services/auth/repository/redis_repository.go` (create)

### Steps
1. Store sessions in Redis
2. Track session metadata (IP, device, location)
3. Implement session expiry
4. Add single sign-out
5. List active sessions

### Testing
```bash
# Get active sessions
curl -X GET localhost:8081/auth/sessions \
  -H "Authorization: Bearer <token>"

# Revoke session
curl -X DELETE localhost:8081/auth/sessions/<session_id> \
  -H "Authorization: Bearer <token>"
```

### Acceptance Criteria
- ✅ Sessions stored in Redis
- ✅ Session expiry works
- ✅ Single sign-out works
- ✅ Can list active sessions
- ✅ Can revoke specific sessions

---

## AUTH-008: Implement API Key Management
**Priority**: P2 | **Hours**: 3 | **Dependencies**: AUTH-001, DATA-002

### Context
Support API key authentication for programmatic access.

### Implementation
**Files to modify:**
- `internal/services/auth/apikey/manager.go` (create)
- `internal/services/auth/middleware/apikey_middleware.go` (create)

### Steps
1. Generate API keys
2. Store hashed keys
3. Set expiry dates
4. Track usage
5. Implement rate limiting per key

### Testing
```bash
# Generate API key
curl -X POST localhost:8081/auth/apikeys \
  -H "Authorization: Bearer <token>" \
  -d '{"name":"CI/CD Key","expires_in_days":90}'

# Use API key
curl -X GET localhost:8081/api/workflows \
  -H "X-API-Key: <api_key>"
```

### Acceptance Criteria
- ✅ API keys generated
- ✅ Keys properly hashed
- ✅ Expiry enforced
- ✅ Usage tracked
- ✅ Rate limiting works

---

## AUTH-009: Implement Password Reset Flow
**Priority**: P1 | **Hours**: 3 | **Dependencies**: AUTH-002, NOTIF-001

### Context
Users need ability to reset forgotten passwords.

### Implementation
**Files to modify:**
- `internal/services/auth/handlers/password_reset.go` (create)
- `internal/services/auth/service/password_service.go` (create)

### Steps
1. Generate reset tokens
2. Send reset emails
3. Validate reset tokens
4. Update passwords
5. Invalidate sessions

### Testing
```bash
# Request reset
curl -X POST localhost:8081/auth/password/reset \
  -d '{"email":"user@example.com"}'

# Reset password
curl -X POST localhost:8081/auth/password/update \
  -d '{"token":"<token>","new_password":"NewPass123!"}'
```

### Acceptance Criteria
- ✅ Reset emails sent
- ✅ Tokens expire after 1 hour
- ✅ Password updated successfully
- ✅ Old sessions invalidated
- ✅ Cannot reuse tokens

---

## AUTH-010: Implement Auth Service Middleware
**Priority**: P0 | **Hours**: 3 | **Dependencies**: AUTH-001

### Context
Create reusable middleware for authentication across all services.

### Implementation
**Files to modify:**
- `pkg/middleware/auth/jwt_middleware.go` (create)
- `pkg/middleware/auth/permission_middleware.go` (create)
- `internal/services/*/server/server.go` (update all)

### Steps
1. Create JWT validation middleware
2. Create permission checking middleware
3. Add user context injection
4. Implement service-to-service auth
5. Add to all service routers

### Testing
```bash
# Test protected endpoint without token
curl -X GET localhost:8082/api/users/profile
# Should return 401

# Test with valid token
curl -X GET localhost:8082/api/users/profile \
  -H "Authorization: Bearer <token>"
# Should return profile
```

### Acceptance Criteria
- ✅ Middleware validates JWT
- ✅ Invalid tokens rejected
- ✅ User context available in handlers
- ✅ Service-to-service auth works
- ✅ Applied to all services

---

## Summary Stats
- **Total Tasks**: 10
- **Total Hours**: 38
- **Critical (P0)**: 3
- **High (P1)**: 4
- **Medium (P2)**: 3

## Execution Order
1. AUTH-001, AUTH-002, AUTH-010 (parallel)
2. AUTH-003, AUTH-005, AUTH-007
3. AUTH-004, AUTH-009
4. AUTH-006, AUTH-008

## Team Assignment Suggestion
- **Senior Dev**: AUTH-001, AUTH-005, AUTH-010
- **Mid Dev 1**: AUTH-002, AUTH-003, AUTH-009
- **Mid Dev 2**: AUTH-004, AUTH-006, AUTH-007, AUTH-008
