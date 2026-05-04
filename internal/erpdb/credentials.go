package erpdb

import (
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"
)

func (r *Reader) AdminAPIAuth(ctx context.Context, username string) (string, string, error) {
	user := strings.TrimSpace(username)
	if user == "" {
		user = "Administrator"
	}

	var apiKey string
	if err := r.db.QueryRowContext(ctx, `
		SELECT COALESCE(api_key, '')
		FROM tabUser
		WHERE name = ?
		LIMIT 1`,
		user,
	).Scan(&apiKey); err != nil {
		return "", "", err
	}
	apiKey = strings.TrimSpace(apiKey)
	if apiKey == "" {
		return "", "", fmt.Errorf("api key not configured for %s", user)
	}

	var encryptedSecret string
	if err := r.db.QueryRowContext(ctx, `
		SELECT password
		FROM __Auth
		WHERE doctype = 'User'
		  AND name = ?
		  AND fieldname = 'api_secret'
		  AND encrypted = 1
		LIMIT 1`,
		user,
	).Scan(&encryptedSecret); err != nil {
		return "", "", err
	}

	secret, err := decryptFernet(strings.TrimSpace(encryptedSecret), r.encryptionKey)
	if err != nil {
		return "", "", err
	}
	if strings.TrimSpace(secret) == "" {
		return "", "", fmt.Errorf("api secret not configured for %s", user)
	}
	return apiKey, secret, nil
}

func (r *Reader) UpdateAdminAPIAuth(ctx context.Context, username, apiKey, apiSecret string) error {
	user := strings.TrimSpace(username)
	if user == "" {
		user = "Administrator"
	}
	apiKey = strings.TrimSpace(apiKey)
	apiSecret = strings.TrimSpace(apiSecret)
	if apiKey == "" {
		return fmt.Errorf("api key is required")
	}
	if apiSecret == "" {
		return fmt.Errorf("api secret is required")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			_ = tx.Rollback()
		}
	}()

	if _, err = tx.ExecContext(ctx, `
		UPDATE tabUser
		SET api_key = ?
		WHERE name = ?`,
		apiKey, user,
	); err != nil {
		return err
	}

	encryptedSecret, err := encryptFernet(apiSecret, r.encryptionKey)
	if err != nil {
		return err
	}

	if _, err = tx.ExecContext(ctx, `
		INSERT INTO __Auth (doctype, name, fieldname, password, encrypted)
		VALUES ('User', ?, 'api_secret', ?, 1)
		ON DUPLICATE KEY UPDATE password = VALUES(password), encrypted = VALUES(encrypted)`,
		user, encryptedSecret,
	); err != nil {
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	return nil
}

func encryptFernet(plaintext, encryptionKey string) (string, error) {
	key, err := decodeFernetKey(encryptionKey)
	if err != nil {
		return "", err
	}
	signingKey := key[:16]
	encKey := key[16:]

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return "", err
	}

	iv := make([]byte, aes.BlockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	padded := pkcs7Pad([]byte(plaintext), aes.BlockSize)
	ciphertext := make([]byte, len(padded))
	cipher.NewCBCEncrypter(block, iv).CryptBlocks(ciphertext, padded)

	payload := make([]byte, 0, 1+8+len(iv)+len(ciphertext)+sha256.Size)
	payload = append(payload, 0x80)
	ts := make([]byte, 8)
	binary.BigEndian.PutUint64(ts, uint64(time.Now().Unix()))
	payload = append(payload, ts...)
	payload = append(payload, iv...)
	payload = append(payload, ciphertext...)

	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write(payload)
	payload = append(payload, mac.Sum(nil)...)

	return base64.URLEncoding.EncodeToString(payload), nil
}

func decryptFernet(token, encryptionKey string) (string, error) {
	key, err := decodeFernetKey(encryptionKey)
	if err != nil {
		return "", err
	}
	signingKey := key[:16]
	encKey := key[16:]

	raw, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", err
	}
	if len(raw) < 1+8+aes.BlockSize+sha256.Size || raw[0] != 0x80 {
		return "", fmt.Errorf("invalid fernet token")
	}

	macOffset := len(raw) - sha256.Size
	payload := raw[:macOffset]
	expectedMAC := raw[macOffset:]
	mac := hmac.New(sha256.New, signingKey)
	_, _ = mac.Write(payload)
	if !hmac.Equal(mac.Sum(nil), expectedMAC) {
		return "", fmt.Errorf("invalid fernet token signature")
	}

	iv := raw[1+8 : 1+8+aes.BlockSize]
	ciphertext := raw[1+8+aes.BlockSize : macOffset]
	if len(ciphertext)%aes.BlockSize != 0 {
		return "", fmt.Errorf("invalid fernet ciphertext size")
	}

	block, err := aes.NewCipher(encKey)
	if err != nil {
		return "", err
	}
	plaintext := make([]byte, len(ciphertext))
	cipher.NewCBCDecrypter(block, iv).CryptBlocks(plaintext, ciphertext)

	unpadded, err := pkcs7Unpad(plaintext, aes.BlockSize)
	if err != nil {
		return "", err
	}
	return string(unpadded), nil
}

func decodeFernetKey(encryptionKey string) ([]byte, error) {
	key := strings.TrimSpace(encryptionKey)
	if key == "" {
		return nil, fmt.Errorf("encryption key is required")
	}
	decoded, err := base64.URLEncoding.DecodeString(key)
	if err != nil {
		return nil, err
	}
	if len(decoded) != 32 {
		return nil, fmt.Errorf("invalid encryption key length")
	}
	return decoded, nil
}

func pkcs7Pad(src []byte, blockSize int) []byte {
	if blockSize <= 0 {
		return src
	}
	padding := blockSize - (len(src) % blockSize)
	if padding == 0 {
		padding = blockSize
	}
	return append(src, bytes.Repeat([]byte{byte(padding)}, padding)...)
}

func pkcs7Unpad(src []byte, blockSize int) ([]byte, error) {
	if blockSize <= 0 || len(src) == 0 || len(src)%blockSize != 0 {
		return nil, errors.New("invalid padding")
	}
	padding := int(src[len(src)-1])
	if padding == 0 || padding > blockSize || padding > len(src) {
		return nil, errors.New("invalid padding")
	}
	for i := len(src) - padding; i < len(src); i++ {
		if int(src[i]) != padding {
			return nil, errors.New("invalid padding")
		}
	}
	return src[:len(src)-padding], nil
}
