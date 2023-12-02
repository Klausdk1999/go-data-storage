package main

import (
	"encoding/json"
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
    defer db.Close()

    rows, err := db.Query("SELECT id, name FROM users")
    if err != nil {
        http.Error(w, "Database query error", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var users []User
    for rows.Next() {
        var user User
        err := rows.Scan(&user.ID, &user.Name)
        if err != nil {
            http.Error(w, "Error scanning users", http.StatusInternalServerError)
            return
        }
        users = append(users, user)
    }

    usersBytes, err := json.MarshalIndent(users, "", "\t")
    if err != nil {
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

    sqlStatement := `INSERT INTO users (name, rfid) VALUES ($1, $2)`
    _, err = db.Exec(sqlStatement, user.Name, user.Rfid)
    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}
