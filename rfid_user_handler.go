package main

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
	_ "github.com/lib/pq"
)

// GetUserByRFIDHandler handles requests to get a user by RFID
func GetUserByRFIDHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
		return
	}

	db := OpenConnection()
	defer db.Close()

	// Extract RFID from URL path using mux
	vars := mux.Vars(r)
	rfid, ok := vars["rfid"]
	if !ok {
		http.Error(w, "RFID is required", http.StatusBadRequest)
		return
	}

	row := db.QueryRow("SELECT id, name, categoria, matricula, rfid FROM users WHERE rfid = $1", rfid)
	
	var user User
	err := row.Scan(&user.ID, &user.Name, &user.Rfid, &user.Categoria, &user.Matricula)
	if err != nil {
		if err == pgx.ErrNoRows {
			http.Error(w, "User not found", http.StatusNotFound)
		} else {
			http.Error(w, "Database query error", http.StatusInternalServerError)
		}
		return
	}

	userBytes, err := json.MarshalIndent(user, "", "\t")
	if err != nil {
		http.Error(w, "Error marshaling user data", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(userBytes)
}
