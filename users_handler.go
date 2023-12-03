package main

import (
	"encoding/json"
	"log"
	"net/http"
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
    db := OpenConnection()
    if db == nil {
        http.Error(w, "Failed to open database connection", http.StatusInternalServerError)
        return
    }
    defer db.Close()

    rows, err := db.Query("SELECT id, name, categoria, matricula, rfid FROM users")
    if err != nil {
        log.Printf("Database query error: %v", err)
        http.Error(w, "Database query error", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        var user User
        err := rows.Scan(&user.ID, &user.Name, &user.Categoria, &user.Matricula, &user.Rfid)
        if err != nil {
            log.Printf("Error scanning users: %v", err)
            http.Error(w, "Error scanning users", http.StatusInternalServerError)
            return
        }
        users = append(users, user)
    }

    // Check for errors during iteration
    if err = rows.Err(); err != nil {
        log.Printf("Error iterating over rows: %v", err)
        http.Error(w, "Error iterating over rows", http.StatusInternalServerError)
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
    db := OpenConnection()
    defer db.Close()

    var user User
    err := json.NewDecoder(r.Body).Decode(&user)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    sqlStatement := `INSERT INTO users (name, rfid, categoria, matricula) VALUES ($1, $2, $3, $4);`
    _, err = db.Exec(sqlStatement, user.Name, user.Rfid, user.Categoria, user.Matricula)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}
