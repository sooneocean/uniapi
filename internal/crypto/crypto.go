package crypto

import (
    "crypto/aes"
    "crypto/cipher"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "errors"
    "fmt"
    "io"
    "os"
    "strings"

    "golang.org/x/crypto/hkdf"
)

func DeriveKeyWithInfo(secret string, info string) ([]byte, error) {
    hkdfReader := hkdf.New(sha256.New, []byte(secret), []byte("uniapi-salt"), []byte(info))
    key := make([]byte, 32)
    if _, err := io.ReadFull(hkdfReader, key); err != nil {
        return nil, fmt.Errorf("hkdf key derivation failed: %w", err)
    }
    return key, nil
}

func DeriveKey(secret string) ([]byte, error) {
    return DeriveKeyWithInfo(secret, "uniapi-encryption")
}

func Encrypt(key []byte, plaintext string) (string, error) {
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    nonce := make([]byte, gcm.NonceSize())
    if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
        return "", err
    }
    ciphertext := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
    return hex.EncodeToString(ciphertext), nil
}

func Decrypt(key []byte, ciphertextHex string) (string, error) {
    ciphertext, err := hex.DecodeString(ciphertextHex)
    if err != nil {
        return "", err
    }
    block, err := aes.NewCipher(key)
    if err != nil {
        return "", err
    }
    gcm, err := cipher.NewGCM(block)
    if err != nil {
        return "", err
    }
    nonceSize := gcm.NonceSize()
    if len(ciphertext) < nonceSize {
        return "", errors.New("ciphertext too short")
    }
    nonce, ciphertext := ciphertext[:nonceSize], ciphertext[nonceSize:]
    plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
    if err != nil {
        return "", err
    }
    return string(plaintext), nil
}

func LoadOrCreateSecret(path string) (string, error) {
    data, err := os.ReadFile(path)
    if err == nil {
        return strings.TrimSpace(string(data)), nil
    }
    if !os.IsNotExist(err) {
        return "", err
    }
    key := make([]byte, 32)
    if _, err := io.ReadFull(rand.Reader, key); err != nil {
        return "", err
    }
    secret := hex.EncodeToString(key)
    if err := os.WriteFile(path, []byte(secret), 0600); err != nil {
        return "", err
    }
    return secret, nil
}
