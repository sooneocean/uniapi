package crypto

import (
    "os"
    "path/filepath"
    "testing"
)

func TestDeriveKey(t *testing.T) {
    key := DeriveKey("my-secret-password")
    if len(key) != 32 {
        t.Errorf("expected 32-byte key, got %d", len(key))
    }
    key2 := DeriveKey("my-secret-password")
    if string(key) != string(key2) {
        t.Error("same input should produce same key")
    }
    key3 := DeriveKey("different-password")
    if string(key) == string(key3) {
        t.Error("different input should produce different key")
    }
}

func TestEncryptDecrypt(t *testing.T) {
    key := DeriveKey("test-secret")
    plaintext := "sk-ant-api-key-12345"
    ciphertext, err := Encrypt(key, plaintext)
    if err != nil {
        t.Fatalf("encrypt failed: %v", err)
    }
    if ciphertext == plaintext {
        t.Error("ciphertext should differ from plaintext")
    }
    decrypted, err := Decrypt(key, ciphertext)
    if err != nil {
        t.Fatalf("decrypt failed: %v", err)
    }
    if decrypted != plaintext {
        t.Errorf("expected %q, got %q", plaintext, decrypted)
    }
}

func TestDecryptWrongKey(t *testing.T) {
    key1 := DeriveKey("secret-1")
    key2 := DeriveKey("secret-2")
    ciphertext, err := Encrypt(key1, "sensitive data")
    if err != nil {
        t.Fatal(err)
    }
    _, err = Decrypt(key2, ciphertext)
    if err == nil {
        t.Error("decrypt with wrong key should fail")
    }
}

func TestLoadOrCreateSecret(t *testing.T) {
    dir := t.TempDir()
    secretPath := filepath.Join(dir, "secret")
    secret1, err := LoadOrCreateSecret(secretPath)
    if err != nil {
        t.Fatal(err)
    }
    if len(secret1) == 0 {
        t.Error("secret should not be empty")
    }
    secret2, err := LoadOrCreateSecret(secretPath)
    if err != nil {
        t.Fatal(err)
    }
    if secret1 != secret2 {
        t.Error("should return same secret on second call")
    }
    if _, err := os.Stat(secretPath); os.IsNotExist(err) {
        t.Error("secret file should exist")
    }
}
