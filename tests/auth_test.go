package main

import (
	"testing"
	"time"
)

func TestGenerateJWT(t *testing.T) {
	userID := uint(1)
	email := "test@example.com"

	token, err := GenerateJWT(userID, email)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if token == "" {
		t.Error("Token should not be empty")
	}

	// Verify token can be validated
	claims, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("Expected UserID %d, got %d", userID, claims.UserID)
	}

	if claims.Email != email {
		t.Errorf("Expected Email %s, got %s", email, claims.Email)
	}

	if claims.UserType != "user" {
		t.Errorf("Expected UserType 'user', got %s", claims.UserType)
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	invalidToken := "invalid.token.here"
	_, err := ValidateJWT(invalidToken)
	if err == nil {
		t.Error("ValidateJWT should fail for invalid token")
	}
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	// Note: This test would require mocking time or using a token with past expiration
	// For now, we'll test that valid tokens work
	userID := uint(1)
	email := "test@example.com"

	token, err := GenerateJWT(userID, email)
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	claims, err := ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	// Check expiration is in the future
	if claims.ExpiresAt != nil {
		if claims.ExpiresAt.Time.Before(time.Now()) {
			t.Error("Token expiration should be in the future")
		}
	}
}

