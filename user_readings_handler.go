package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

func UserReadingsHandler(w http.ResponseWriter, r *http.Request) {
  if r.Method != "GET" {
        http.Error(w, "Only GET method is allowed", http.StatusMethodNotAllowed)
        return
    }


    db := OpenConnection() // Ensure this function exists and properly opens a database connection
    defer db.Close()

    // Extract user ID from URL path using mux
    vars := mux.Vars(r)
    userIDStr, ok := vars["user_id"]
    if !ok {
        http.Error(w, "User ID is required", http.StatusBadRequest)
        return
    }

    userID, err := strconv.Atoi(userIDStr)
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }

    // Query database for readings belonging to the user
    rows, err := db.Query("SELECT id, userid, timestamp, value, torquevalues, motionwastes, setvalue, asmtimes FROM readings WHERE user_id = $1", userID)
    if err != nil {
        http.Error(w, "Database query error", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

 var readings []Reading
    for rows.Next() {
        var reading Reading
        err := rows.Scan(&reading.ID, &reading.UserID, &reading.Timestamp, &reading.Value, pq.Array(&reading.TorqueValues), pq.Array(&reading.MotionWastes), &reading.SetValue, pq.Array(&reading.AsmTimes))

        if err != nil {
            http.Error(w, "Error scanning readings", http.StatusInternalServerError)
            return
        }
        readings = append(readings, reading)
    }

    readingsBytes, err := json.MarshalIndent(readings, "", "\t")
    if err != nil {
        http.Error(w, "Error marshaling readings", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(readingsBytes)
}