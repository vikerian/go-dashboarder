package domain

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
)

// SignMessage vytvoří HMAC podpis pro daný payload pomocí tajného klíče.
// Musí začínat velkým písmenem, aby byla viditelná z jiných balíčků!
func SignMessage(payload []byte, secretKey string) string {
	h := hmac.New(sha256.New, []byte(secretKey))
	h.Write(payload)
	return hex.EncodeToString(h.Sum(nil))
}

// VerifyHMAC zkontroluje, zda podpis odpovídá payloadu a klíči.
func VerifyHMAC(payload []byte, receivedSignature string, secretKey string) bool {
	expectedSignature := SignMessage(payload, secretKey)
	// hmac.Equal je odolný proti timing attacks
	return hmac.Equal([]byte(expectedSignature), []byte(receivedSignature))
}
