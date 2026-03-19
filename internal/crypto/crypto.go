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

func DeriveKey(secret string) []byte {
    hkdfReader := hkdf.New(sha256.New, []byte(secret), []byte("uniapi-salt"), []byte("uniapi-encryption"))
    key := make([]byte, 32)
    if _, err := io.ReadFull(hkdfReader, key); err != nil {
        panic(fmt.Sprintf("hkdf failed: %v", err))
    }
    return key
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
