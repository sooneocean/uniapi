package auth

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
    "time"

    "github.com/golang-jwt/jwt/v5"
    "golang.org/x/crypto/bcrypt"
)

// HashPassword returns a bcrypt hash of the given password.
func HashPassword(password string) (string, error) {
    hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
    if err != nil {
        return "", err
    }
    return string(hash), nil
}

// VerifyPassword checks a plaintext password against a bcrypt hash.
func VerifyPassword(hash, password string) bool {
    return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Claims holds the JWT payload for authenticated users.
type Claims struct {
    UserID string `json:"uid"`
    Role   string `json:"role"`
    jwt.RegisteredClaims
}

// JWTManager creates and validates signed JWT tokens.
type JWTManager struct {
    secret   []byte
    lifetime time.Duration
}

// NewJWTManager creates a JWTManager with the given HMAC secret and token lifetime.
func NewJWTManager(secret []byte, lifetime time.Duration) *JWTManager {
    return &JWTManager{secret: secret, lifetime: lifetime}
}

func (j *JWTManager) CreateToken(userID, role string) (string, error) {
    claims := Claims{
        UserID: userID,
        Role:   role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(j.lifetime)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }
    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString(j.secret)
}

func (j *JWTManager) ParseToken(tokenStr string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(t *jwt.Token) (interface{}, error) {
        if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
            return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
        }
        return j.secret, nil
    })
    if err != nil {
        return nil, err
    }
    claims, ok := token.Claims.(*Claims)
    if !ok || !token.Valid {
        return nil, fmt.Errorf("invalid token")
    }
    return claims, nil
}

// GenerateAPIKey generates a random uniapi-sk- prefixed API key.
func GenerateAPIKey() (string, error) {
    b := make([]byte, 32)
    if _, err := rand.Read(b); err != nil {
        return "", fmt.Errorf("generate API key: %w", err)
    }
    return "uniapi-sk-" + hex.EncodeToString(b), nil
}

// HashAPIKey returns the SHA-256 hex hash of an API key for safe storage.
func HashAPIKey(key string) string {
    h := sha256.Sum256([]byte(key))
    return hex.EncodeToString(h[:])
}
