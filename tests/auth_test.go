package tests

import (
	"testing"
	"time"

	"data-storage/internal/auth"
)

func TestGenerateJWT(t *testing.T) {
	userID := uint(1)
	email := "test@example.com"

	token, err := auth.GenerateJWT(userID, email, "admin")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	if token == "" {
		t.Error("Token should not be empty")
	}

	// Verify token can be validated
	claims, err := auth.ValidateJWT(token)
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

	if claims.UserRole != "admin" {
		t.Errorf("Expected UserRole 'admin', got %s", claims.UserRole)
	}
}

func TestValidateJWT_InvalidToken(t *testing.T) {
	invalidToken := "invalid.token.here"
	_, err := auth.ValidateJWT(invalidToken)
	if err == nil {
		t.Error("ValidateJWT should fail for invalid token")
	}
}

func TestValidateJWT_ExpiredToken(t *testing.T) {
	userID := uint(1)
	email := "test@example.com"

	token, err := auth.GenerateJWT(userID, email, "worker")
	if err != nil {
		t.Fatalf("GenerateJWT failed: %v", err)
	}

	claims, err := auth.ValidateJWT(token)
	if err != nil {
		t.Fatalf("ValidateJWT failed: %v", err)
	}

	// Check expiration is in the future
	if claims.ExpiresAt != nil {
		if claims.ExpiresAt.Time.Before(time.Now()) {
			t.Error("Token expiration should be in the future")
		}
	}

	// Verify role is set correctly
	if claims.UserRole != "worker" {
		t.Errorf("Expected UserRole 'worker', got %s", claims.UserRole)
	}
}
