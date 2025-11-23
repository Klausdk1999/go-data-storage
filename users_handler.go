package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

func UsersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllUsers(w, r)
	case "POST":
		createUser(w, r)
	default:
		http.Error(w, "Unsupported request method.", http.StatusMethodNotAllowed)
	}
}

func getAllUsers(w http.ResponseWriter, r *http.Request) {
	var users []User
	result := DB.Find(&users)
	if result.Error != nil {
		log.Printf("Database query error: %v", result.Error)
		http.Error(w, "Database query error", http.StatusInternalServerError)
		return
	}

	usersBytes, err := json.MarshalIndent(users, "", "\t")
	if err != nil {
		log.Printf("Error marshaling users: %v", err)
		http.Error(w, "Error marshaling users", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(usersBytes)
}

func createUser(w http.ResponseWriter, r *http.Request) {
	var user User
	var userData struct {
		Name      string `json:"name"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		Categoria string `json:"categoria"`
		Matricula string `json:"matricula"`
		Rfid      string `json:"rfid"`
	}

	err := json.NewDecoder(r.Body).Decode(&userData)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user.Name = userData.Name
	user.Email = userData.Email
	user.Categoria = userData.Categoria
	user.Matricula = userData.Matricula
	user.Rfid = userData.Rfid

	// Hash password if provided
	if userData.Password != "" {
		if err := user.SetPassword(userData.Password); err != nil {
			log.Printf("Error hashing password: %v", err)
			http.Error(w, "Error processing password", http.StatusInternalServerError)
			return
		}
	}

	result := DB.Create(&user)
	if result.Error != nil {
		log.Printf("Error creating user: %v", result.Error)
		http.Error(w, result.Error.Error(), http.StatusInternalServerError)
		return
	}

	// Clear password hash from response
	user.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(user)
}

// UserHandler handles individual user operations
func UserHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getUser(w, r)
	case "PUT":
		updateUser(w, r)
	case "DELETE":
		deleteUser(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var user User
	result := DB.Preload("Devices").First(&user, userID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching user: %v", result.Error)
			http.Error(w, "Error fetching user", http.StatusInternalServerError)
		}
		return
	}

	// Clear password hash
	user.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var user User
	result := DB.First(&user, userID)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching user", http.StatusInternalServerError)
		}
		return
	}

	var updateData struct {
		Name      string `json:"name"`
		Email     string `json:"email"`
		Password  string `json:"password"`
		Categoria string `json:"categoria"`
		Matricula string `json:"matricula"`
		Rfid      string `json:"rfid"`
		IsActive  *bool  `json:"is_active"`
	}

	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update fields
	if updateData.Name != "" {
		user.Name = updateData.Name
	}
	if updateData.Email != "" {
		user.Email = updateData.Email
	}
	if updateData.Categoria != "" {
		user.Categoria = updateData.Categoria
	}
	if updateData.Matricula != "" {
		user.Matricula = updateData.Matricula
	}
	if updateData.Rfid != "" {
		user.Rfid = updateData.Rfid
	}
	if updateData.IsActive != nil {
		user.IsActive = *updateData.IsActive
	}
	if updateData.Password != "" {
		if err := user.SetPassword(updateData.Password); err != nil {
			log.Printf("Error hashing password: %v", err)
			http.Error(w, "Error processing password", http.StatusInternalServerError)
			return
		}
	}

	result = DB.Save(&user)
	if result.Error != nil {
		log.Printf("Error updating user: %v", result.Error)
		http.Error(w, "Error updating user", http.StatusInternalServerError)
		return
	}

	// Clear password hash
	user.PasswordHash = ""

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func deleteUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userIDStr, ok := vars["id"]
	if !ok {
		http.Error(w, "User ID required", http.StatusBadRequest)
		return
	}

	userID, err := strconv.ParseUint(userIDStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	result := DB.Delete(&User{}, userID)
	if result.Error != nil {
		log.Printf("Error deleting user: %v", result.Error)
		http.Error(w, "Error deleting user", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

