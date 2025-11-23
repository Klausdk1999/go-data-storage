package handlers

import (
	"encoding/json"
	"net/http"

	"data-storage/internal/db"
	"data-storage/internal/models"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

// GetUserByRFIDHandler handles requests to get a user by RFID
func GetUserByRFIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	rfid, ok := vars["rfid"]
	if !ok {
		http.Error(w, "RFID is required", http.StatusBadRequest)
		return
	}

	var user models.User
	result := db.GetDB().Where("rfid = ?", rfid).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database query error", http.StatusInternalServerError)
		}
		return
	}

	// Clear password hash
	user.PasswordHash = ""

	userBytes, err := json.MarshalIndent(user, "", "\t")
	if err != nil {
		http.Error(w, "Error marshaling user data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(userBytes)
}

