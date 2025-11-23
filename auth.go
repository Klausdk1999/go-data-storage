package main

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"gorm.io/gorm"
)

var jwtSecret = []byte("your-secret-key-change-in-production") // TODO: Move to env var

type Claims struct {
	UserID   uint   `json:"user_id"`
	Email    string `json:"email"`
	UserType string `json:"user_type"` // "user" or "device"
	DeviceID uint   `json:"device_id,omitempty"`
	jwt.RegisteredClaims
}

// GenerateDeviceToken generates a secure random token for device authentication
func GenerateDeviceToken() (string, error) {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GenerateJWT generates a JWT token for a user
func GenerateJWT(userID uint, email string) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID:   userID,
		Email:    email,
		UserType: "user",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// ValidateJWT validates a JWT token and returns the claims
func ValidateJWT(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

// AuthenticateDevice validates a device auth token
func AuthenticateDevice(authToken string) (*Device, error) {
	var device Device
	result := DB.Where("auth_token = ? AND is_active = ?", authToken, true).First(&device)
	if result.Error != nil {
		return nil, result.Error
	}
	return &device, nil
}

// Middleware: RequireUserAuth requires a valid JWT token
func RequireUserAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		claims, err := ValidateJWT(parts[1])
		if err != nil {
			http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
			return
		}

		if claims.UserType != "user" {
			http.Error(w, "Invalid token type", http.StatusUnauthorized)
			return
		}

		// Store user info in request context (can be accessed in handlers)
		r.Header.Set("X-User-ID", strconv.FormatUint(uint64(claims.UserID), 10))
		r.Header.Set("X-User-Email", claims.Email)

		next(w, r)
	}
}

// Middleware: RequireDeviceAuth requires a valid device auth token
func RequireDeviceAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		device, err := AuthenticateDevice(parts[1])
		if err != nil {
			if err == gorm.ErrRecordNotFound {
				http.Error(w, "Invalid device token", http.StatusUnauthorized)
			} else {
				log.Printf("Error authenticating device: %v", err)
				http.Error(w, "Authentication error", http.StatusInternalServerError)
			}
			return
		}

		// Store device info in request context
		r.Header.Set("X-Device-ID", strconv.FormatUint(uint64(device.ID), 10))
		if device.UserID != nil {
			r.Header.Set("X-Device-User-ID", strconv.FormatUint(uint64(*device.UserID), 10))
		}

		next(w, r)
	}
}

// Middleware: RequireAnyAuth accepts either user JWT or device token
func RequireAnyAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, "Authorization header required", http.StatusUnauthorized)
			return
		}

		parts := strings.Split(authHeader, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			http.Error(w, "Invalid authorization header format", http.StatusUnauthorized)
			return
		}

		token := parts[1]

		// Try JWT first (user auth)
		claims, err := ValidateJWT(token)
		if err == nil && claims.UserType == "user" {
			r.Header.Set("X-User-ID", strconv.FormatUint(uint64(claims.UserID), 10))
			r.Header.Set("X-User-Email", claims.Email)
			r.Header.Set("X-Auth-Type", "user")
			next(w, r)
			return
		}

		// Try device token
		device, err := AuthenticateDevice(token)
		if err == nil {
			r.Header.Set("X-Device-ID", strconv.FormatUint(uint64(device.ID), 10))
			if device.UserID != nil {
				r.Header.Set("X-Device-User-ID", strconv.FormatUint(uint64(*device.UserID), 10))
			}
			r.Header.Set("X-Auth-Type", "device")
			next(w, r)
			return
		}

		http.Error(w, "Invalid or expired token", http.StatusUnauthorized)
	}
}

