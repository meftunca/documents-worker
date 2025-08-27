package security

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rs/zerolog"
)

// SecurityConfig holds security configuration
type SecurityConfig struct {
	// JWT Configuration
	JWTSecret         string        `json:"jwt_secret" validate:"required,min=32"`
	JWTIssuer         string        `json:"jwt_issuer" validate:"required"`
	JWTAudience       string        `json:"jwt_audience" validate:"required"`
	JWTExpirationTime time.Duration `json:"jwt_expiration" validate:"min=5m"`
	JWTRefreshTime    time.Duration `json:"jwt_refresh" validate:"min=1h"`
	JWTClockSkew      time.Duration `json:"jwt_clock_skew"`

	// API Key Configuration
	APIKeyHeader      string        `json:"api_key_header"`
	APIKeyLength      int           `json:"api_key_length" validate:"min=16,max=128"`
	APIKeyExpiration  time.Duration `json:"api_key_expiration"`
	MaxAPIKeysPerUser int           `json:"max_api_keys_per_user" validate:"min=1,max=50"`

	// Rate Limiting
	EnableRateLimit  bool          `json:"enable_rate_limit"`
	DefaultRateLimit int           `json:"default_rate_limit" validate:"min=1"`
	RateLimitWindow  time.Duration `json:"rate_limit_window" validate:"min=1s"`
	RateLimitBurst   int           `json:"rate_limit_burst" validate:"min=1"`

	// Security Headers
	EnableSecurityHeaders bool     `json:"enable_security_headers"`
	AllowedOrigins        []string `json:"allowed_origins"`
	AllowedMethods        []string `json:"allowed_methods"`
	AllowedHeaders        []string `json:"allowed_headers"`

	// Password Policy
	MinPasswordLength   int  `json:"min_password_length" validate:"min=8,max=128"`
	RequireUppercase    bool `json:"require_uppercase"`
	RequireLowercase    bool `json:"require_lowercase"`
	RequireNumbers      bool `json:"require_numbers"`
	RequireSpecialChars bool `json:"require_special_chars"`

	// Session Management
	SessionTimeout        time.Duration `json:"session_timeout" validate:"min=5m"`
	MaxConcurrentSessions int           `json:"max_concurrent_sessions" validate:"min=1,max=100"`

	// Security Features
	EnableBruteForceProtection bool          `json:"enable_brute_force_protection"`
	MaxLoginAttempts           int           `json:"max_login_attempts" validate:"min=3,max=10"`
	LockoutDuration            time.Duration `json:"lockout_duration" validate:"min=5m"`
}

// DefaultSecurityConfig returns default security configuration
func DefaultSecurityConfig() *SecurityConfig {
	return &SecurityConfig{
		JWTSecret:                  generateRandomString(64),
		JWTIssuer:                  "documents-worker",
		JWTAudience:                "documents-worker-api",
		JWTExpirationTime:          24 * time.Hour,
		JWTRefreshTime:             7 * 24 * time.Hour,
		JWTClockSkew:               5 * time.Minute,
		APIKeyHeader:               "X-API-Key",
		APIKeyLength:               32,
		APIKeyExpiration:           365 * 24 * time.Hour,
		MaxAPIKeysPerUser:          10,
		EnableRateLimit:            true,
		DefaultRateLimit:           100,
		RateLimitWindow:            time.Hour,
		RateLimitBurst:             10,
		EnableSecurityHeaders:      true,
		AllowedOrigins:             []string{"*"},
		AllowedMethods:             []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:             []string{"Content-Type", "Authorization", "X-API-Key"},
		MinPasswordLength:          12,
		RequireUppercase:           true,
		RequireLowercase:           true,
		RequireNumbers:             true,
		RequireSpecialChars:        true,
		SessionTimeout:             8 * time.Hour,
		MaxConcurrentSessions:      5,
		EnableBruteForceProtection: true,
		MaxLoginAttempts:           5,
		LockoutDuration:            15 * time.Minute,
	}
}

// User represents a user in the system
type User struct {
	ID           string            `json:"id"`
	Username     string            `json:"username"`
	Email        string            `json:"email"`
	PasswordHash string            `json:"password_hash,omitempty"`
	Roles        []string          `json:"roles"`
	Permissions  []string          `json:"permissions"`
	Metadata     map[string]string `json:"metadata"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	LastLoginAt  *time.Time        `json:"last_login_at,omitempty"`
	IsActive     bool              `json:"is_active"`
	IsVerified   bool              `json:"is_verified"`
}

// APIKey represents an API key
type APIKey struct {
	ID          string            `json:"id"`
	UserID      string            `json:"user_id"`
	Name        string            `json:"name"`
	KeyHash     string            `json:"key_hash"`
	Prefix      string            `json:"prefix"`
	Permissions []string          `json:"permissions"`
	Metadata    map[string]string `json:"metadata"`
	CreatedAt   time.Time         `json:"created_at"`
	ExpiresAt   *time.Time        `json:"expires_at,omitempty"`
	LastUsedAt  *time.Time        `json:"last_used_at,omitempty"`
	IsActive    bool              `json:"is_active"`
}

// Claims represents JWT claims
type Claims struct {
	UserID      string   `json:"user_id"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	Roles       []string `json:"roles"`
	Permissions []string `json:"permissions"`
	SessionID   string   `json:"session_id"`
	jwt.RegisteredClaims
}

// SecurityManager handles authentication and authorization
type SecurityManager struct {
	config        *SecurityConfig
	logger        zerolog.Logger
	users         map[string]*User // In-memory store (would be database in production)
	apiKeys       map[string]*APIKey
	sessions      map[string]*Session
	loginAttempts map[string]*LoginAttempts
	mu            sync.RWMutex
}

// Session represents a user session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
	LastSeen  time.Time `json:"last_seen"`
	UserAgent string    `json:"user_agent"`
	IPAddress string    `json:"ip_address"`
	IsActive  bool      `json:"is_active"`
}

// LoginAttempts tracks failed login attempts for brute force protection
type LoginAttempts struct {
	Username    string     `json:"username"`
	Attempts    int        `json:"attempts"`
	LastAttempt time.Time  `json:"last_attempt"`
	LockedUntil *time.Time `json:"locked_until,omitempty"`
}

// NewSecurityManager creates a new security manager
func NewSecurityManager(config *SecurityConfig, logger zerolog.Logger) *SecurityManager {
	if config == nil {
		config = DefaultSecurityConfig()
	}

	sm := &SecurityManager{
		config:        config,
		logger:        logger.With().Str("component", "security").Logger(),
		users:         make(map[string]*User),
		apiKeys:       make(map[string]*APIKey),
		sessions:      make(map[string]*Session),
		loginAttempts: make(map[string]*LoginAttempts),
	}

	// Start cleanup routine
	go sm.cleanupExpiredSessions()

	sm.logger.Info().
		Bool("rate_limit", config.EnableRateLimit).
		Bool("security_headers", config.EnableSecurityHeaders).
		Bool("brute_force_protection", config.EnableBruteForceProtection).
		Msg("Security manager initialized")

	return sm
}

// CreateUser creates a new user
func (sm *SecurityManager) CreateUser(username, email, password string, roles []string) (*User, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	// Validate password
	if err := sm.validatePassword(password); err != nil {
		return nil, fmt.Errorf("password validation failed: %w", err)
	}

	// Check if user already exists
	for _, user := range sm.users {
		if user.Username == username || user.Email == email {
			return nil, fmt.Errorf("user already exists")
		}
	}

	// Hash password
	passwordHash, err := hashPassword(password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Create user
	user := &User{
		ID:           generateRandomString(16),
		Username:     username,
		Email:        email,
		PasswordHash: passwordHash,
		Roles:        roles,
		Permissions:  sm.getRolePermissions(roles),
		Metadata:     make(map[string]string),
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		IsActive:     true,
		IsVerified:   false,
	}

	sm.users[user.ID] = user

	sm.logger.Info().
		Str("user_id", user.ID).
		Str("username", username).
		Strs("roles", roles).
		Msg("User created")

	return user, nil
}

// Authenticate authenticates a user with username/password
func (sm *SecurityManager) Authenticate(username, password string) (*User, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()

	// Check brute force protection
	if sm.config.EnableBruteForceProtection {
		if locked, until := sm.isUserLocked(username); locked {
			return nil, fmt.Errorf("account locked until %s", until.Format(time.RFC3339))
		}
	}

	// Find user
	var user *User
	for _, u := range sm.users {
		if u.Username == username {
			user = u
			break
		}
	}

	if user == nil {
		sm.recordFailedLogin(username)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Verify password
	if !verifyPassword(password, user.PasswordHash) {
		sm.recordFailedLogin(username)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Reset failed attempts on successful login
	sm.resetFailedLogins(username)

	// Update last login
	now := time.Now()
	user.LastLoginAt = &now

	sm.logger.Info().
		Str("user_id", user.ID).
		Str("username", username).
		Msg("User authenticated")

	return user, nil
}

// GenerateJWT generates a JWT token for a user
func (sm *SecurityManager) GenerateJWT(user *User) (string, error) {
	sessionID := generateRandomString(16)

	// Create session
	session := &Session{
		ID:        sessionID,
		UserID:    user.ID,
		CreatedAt: time.Now(),
		ExpiresAt: time.Now().Add(sm.config.SessionTimeout),
		LastSeen:  time.Now(),
		IsActive:  true,
	}

	sm.mu.Lock()
	sm.sessions[sessionID] = session
	sm.mu.Unlock()

	// Create claims
	claims := &Claims{
		UserID:      user.ID,
		Username:    user.Username,
		Email:       user.Email,
		Roles:       user.Roles,
		Permissions: user.Permissions,
		SessionID:   sessionID,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    sm.config.JWTIssuer,
			Audience:  []string{sm.config.JWTAudience},
			Subject:   user.ID,
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(sm.config.JWTExpirationTime)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	// Create token
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenString, err := token.SignedString([]byte(sm.config.JWTSecret))
	if err != nil {
		return "", fmt.Errorf("failed to sign token: %w", err)
	}

	sm.logger.Info().
		Str("user_id", user.ID).
		Str("session_id", sessionID).
		Msg("JWT token generated")

	return tokenString, nil
}

// ValidateJWT validates a JWT token and returns claims
func (sm *SecurityManager) ValidateJWT(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", token.Header["alg"])
		}
		return []byte(sm.config.JWTSecret), nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to parse token: %w", err)
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("invalid token")
	}

	// Validate session
	sm.mu.RLock()
	session, exists := sm.sessions[claims.SessionID]
	sm.mu.RUnlock()

	if !exists || !session.IsActive || time.Now().After(session.ExpiresAt) {
		return nil, fmt.Errorf("session expired or invalid")
	}

	// Update last seen
	sm.mu.Lock()
	session.LastSeen = time.Now()
	sm.mu.Unlock()

	return claims, nil
}

// Helper functions

func generateRandomString(length int) string {
	bytes := make([]byte, length)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)[:length]
}

func hashPassword(password string) (string, error) {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:]), nil
}

func verifyPassword(password, hash string) bool {
	expectedHash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(expectedHash[:]) == hash
}

func (sm *SecurityManager) validatePassword(password string) error {
	if len(password) < sm.config.MinPasswordLength {
		return fmt.Errorf("password too short")
	}

	hasUpper := false
	hasLower := false
	hasNumber := false
	hasSpecial := false

	for _, char := range password {
		switch {
		case char >= 'A' && char <= 'Z':
			hasUpper = true
		case char >= 'a' && char <= 'z':
			hasLower = true
		case char >= '0' && char <= '9':
			hasNumber = true
		default:
			hasSpecial = true
		}
	}

	if sm.config.RequireUppercase && !hasUpper {
		return fmt.Errorf("password must contain uppercase letters")
	}
	if sm.config.RequireLowercase && !hasLower {
		return fmt.Errorf("password must contain lowercase letters")
	}
	if sm.config.RequireNumbers && !hasNumber {
		return fmt.Errorf("password must contain numbers")
	}
	if sm.config.RequireSpecialChars && !hasSpecial {
		return fmt.Errorf("password must contain special characters")
	}

	return nil
}

func (sm *SecurityManager) getRolePermissions(roles []string) []string {
	permissions := make([]string, 0)

	for _, role := range roles {
		switch role {
		case "admin":
			permissions = append(permissions, "read", "write", "delete", "admin")
		case "user":
			permissions = append(permissions, "read", "write")
		case "viewer":
			permissions = append(permissions, "read")
		}
	}

	return permissions
}

func (sm *SecurityManager) isUserLocked(username string) (bool, *time.Time) {
	attempts, exists := sm.loginAttempts[username]
	if !exists {
		return false, nil
	}

	if attempts.LockedUntil != nil && time.Now().Before(*attempts.LockedUntil) {
		return true, attempts.LockedUntil
	}

	return false, nil
}

func (sm *SecurityManager) recordFailedLogin(username string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()

	attempts, exists := sm.loginAttempts[username]
	if !exists {
		attempts = &LoginAttempts{
			Username: username,
		}
		sm.loginAttempts[username] = attempts
	}

	attempts.Attempts++
	attempts.LastAttempt = time.Now()

	if attempts.Attempts >= sm.config.MaxLoginAttempts {
		lockUntil := time.Now().Add(sm.config.LockoutDuration)
		attempts.LockedUntil = &lockUntil

		sm.logger.Warn().
			Str("username", username).
			Int("attempts", attempts.Attempts).
			Time("locked_until", lockUntil).
			Msg("User account locked due to failed login attempts")
	}
}

func (sm *SecurityManager) resetFailedLogins(username string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	delete(sm.loginAttempts, username)
}

func (sm *SecurityManager) cleanupExpiredSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		sm.mu.Lock()
		now := time.Now()
		for id, session := range sm.sessions {
			if now.After(session.ExpiresAt) {
				delete(sm.sessions, id)
			}
		}
		sm.mu.Unlock()
	}
}

// HasPermission checks if user has a specific permission
func (sm *SecurityManager) HasPermission(user *User, permission string) bool {
	for _, p := range user.Permissions {
		if p == permission {
			return true
		}
	}
	return false
}

// SecurityMiddleware provides HTTP middleware for authentication
func (sm *SecurityManager) SecurityMiddleware() func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Add security headers
			if sm.config.EnableSecurityHeaders {
				w.Header().Set("X-Content-Type-Options", "nosniff")
				w.Header().Set("X-Frame-Options", "DENY")
				w.Header().Set("X-XSS-Protection", "1; mode=block")
				w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			}

			// Extract token from Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				http.Error(w, "Authorization header required", http.StatusUnauthorized)
				return
			}

			tokenString := strings.TrimPrefix(authHeader, "Bearer ")
			if tokenString == authHeader {
				http.Error(w, "Bearer token required", http.StatusUnauthorized)
				return
			}

			// Validate token
			claims, err := sm.ValidateJWT(tokenString)
			if err != nil {
				http.Error(w, "Invalid token", http.StatusUnauthorized)
				return
			}

			// Add user context
			ctx := context.WithValue(r.Context(), "user_claims", claims)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
