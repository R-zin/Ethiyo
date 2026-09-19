package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	// ErrInvalidState indicates the OAuth state parameter is malformed or invalid.
	ErrInvalidState = errors.New("invalid oauth state parameter")

	// ErrExpiredState indicates the state parameter has expired.
	ErrExpiredState = errors.New("oauth state parameter has expired")
)

// GenerateState generates a tamper-proof, time-bound OAuth state string.
// Format: nonce.timestamp.signature
func GenerateState(secret string) (string, error) {
	nonceBytes := make([]byte, 16)
	if _, err := rand.Read(nonceBytes); err != nil {
		return "", fmt.Errorf("failed to generate random state: %w", err)
	}
	nonce := hex.EncodeToString(nonceBytes)
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	payload := fmt.Sprintf("%s.%s", nonce, timestamp)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	return fmt.Sprintf("%s.%s", payload, signature), nil
}

// ValidateState verifies that the provided state string was signed by secret and has not expired.
func ValidateState(state string, secret string, maxAge time.Duration) error {
	parts := strings.Split(state, ".")
	if len(parts) != 3 {
		return ErrInvalidState
	}

	nonce, timestampStr, signature := parts[0], parts[1], parts[2]
	payload := fmt.Sprintf("%s.%s", nonce, timestampStr)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(signature), []byte(expectedSignature)) {
		return ErrInvalidState
	}

	timestamp, err := strconv.ParseInt(timestampStr, 10, 64)
	if err != nil {
		return ErrInvalidState
	}

	stateTime := time.Unix(timestamp, 0)
	if time.Since(stateTime) > maxAge || time.Until(stateTime) > 1*time.Minute {
		return ErrExpiredState
	}

	return nil
}
