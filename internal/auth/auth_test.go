package auth

import (
    "testing"
    "time"
)

func TestHashAndVerifyPassword(t *testing.T) {
    hash, err := HashPassword("mypassword123")
    if err != nil {
        t.Fatal(err)
    }
    if !VerifyPassword(hash, "mypassword123") {
        t.Error("password should verify")
    }
    if VerifyPassword(hash, "wrongpassword") {
        t.Error("wrong password should not verify")
    }
}

func TestJWTCreateAndParse(t *testing.T) {
    secret := []byte("test-secret-key-32-bytes-long!!!")
    jwt := NewJWTManager(secret, 7*24*time.Hour)
    token, err := jwt.CreateToken("user-123", "admin")
    if err != nil {
        t.Fatal(err)
    }
    if token == "" {
        t.Error("token should not be empty")
    }
    claims, err := jwt.ParseToken(token)
    if err != nil {
        t.Fatalf("parse failed: %v", err)
    }
    if claims.UserID != "user-123" {
        t.Errorf("expected user-123, got %s", claims.UserID)
    }
    if claims.Role != "admin" {
        t.Errorf("expected admin, got %s", claims.Role)
    }
}

func TestJWTExpired(t *testing.T) {
    secret := []byte("test-secret-key-32-bytes-long!!!")
    jwt := NewJWTManager(secret, 1*time.Millisecond)
    token, _ := jwt.CreateToken("user-123", "admin")
    time.Sleep(10 * time.Millisecond)
    _, err := jwt.ParseToken(token)
    if err == nil {
        t.Error("expired token should fail")
    }
}

func TestAPIKeyHash(t *testing.T) {
    key := GenerateAPIKey()
    if len(key) < 40 {
        t.Errorf("key too short: %s", key)
    }
    if key[:10] != "uniapi-sk-" {
        t.Errorf("key should start with uniapi-sk-, got %s", key[:10])
    }
    hash := HashAPIKey(key)
    if hash == key {
        t.Error("hash should differ from key")
    }
    if HashAPIKey(key) != hash {
        t.Error("hash should be deterministic")
    }
}
