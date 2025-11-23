package main

import (
	"testing"
)

func TestUser_SetPassword(t *testing.T) {
	user := User{}
	password := "testpassword123"

	err := user.SetPassword(password)
	if err != nil {
		t.Fatalf("SetPassword failed: %v", err)
	}

	if user.PasswordHash == "" {
		t.Error("PasswordHash should not be empty")
	}

	if user.PasswordHash == password {
		t.Error("PasswordHash should be hashed, not plain text")
	}
}

func TestUser_CheckPassword(t *testing.T) {
	user := User{}
	password := "testpassword123"

	err := user.SetPassword(password)
	if err != nil {
		t.Fatalf("SetPassword failed: %v", err)
	}

	// Test correct password
	if !user.CheckPassword(password) {
		t.Error("CheckPassword should return true for correct password")
	}

	// Test incorrect password
	if user.CheckPassword("wrongpassword") {
		t.Error("CheckPassword should return false for incorrect password")
	}
}

func TestGenerateDeviceToken(t *testing.T) {
	token1, err := GenerateDeviceToken()
	if err != nil {
		t.Fatalf("GenerateDeviceToken failed: %v", err)
	}

	if token1 == "" {
		t.Error("Token should not be empty")
	}

	// Generate another token and verify they're different
	token2, err := GenerateDeviceToken()
	if err != nil {
		t.Fatalf("GenerateDeviceToken failed: %v", err)
	}

	if token1 == token2 {
		t.Error("Tokens should be unique")
	}

	// Verify token length (base64 encoded 32 bytes = 44 chars)
	if len(token1) < 40 {
		t.Error("Token should be sufficiently long")
	}
}

