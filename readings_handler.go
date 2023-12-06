package main

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/lib/pq"
	_ "github.com/lib/pq"
)

func ReadingsHandler(w http.ResponseWriter, r *http.Request) {
    switch r.Method {
    case "GET":
        getAllReadings(w, r)
    case "POST":
        createReading(w, r)
    default:
        http.Error(w, "Unsupported request method.", http.StatusMethodNotAllowed)
    }
}

func getAllReadings(w http.ResponseWriter, r *http.Request) {
    db := OpenConnection()
    defer db.Close()

    rows, err := db.Query("SELECT id, userid, value, torquevalues, asmtimes, motionwastes, setvalue FROM readings")
    if err != nil {
        http.Error(w, "Database query error", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var readings []Reading
    for rows.Next() {
        var reading Reading
        err := rows.Scan(reading.ID, reading.UserID, reading.Value, pq.Array(reading.TorqueValues), pq.Array(reading.AsmTimes), pq.Array(reading.MotionWastes), reading.SetValue)
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

func createReading(w http.ResponseWriter, r *http.Request) {
    db := OpenConnection()
    defer db.Close()

    var reading Reading
    err := json.NewDecoder(r.Body).Decode(&reading)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    // Assuming timestamp is auto-generated by the database
    sqlStatement := `INSERT INTO readings (userid, value, torquevalues, asmtimes, motionwastes, setvalue) VALUES ($1, $2, $3, $4, $5, $6)`
    _, err = db.Exec(sqlStatement, reading.UserID, reading.Value, pq.Array(reading.TorqueValues), pq.Array(reading.AsmTimes), pq.Array(reading.MotionWastes), reading.SetValue)

    if err != nil {
        http.Error(w, err.Error(), http.StatusInternalServerError)
        return
    }

    w.WriteHeader(http.StatusOK)
}

func getUserReadings(w http.ResponseWriter, r *http.Request) {
    db := OpenConnection()
    defer db.Close()

    // Get user ID from query parameters
    queryValues := r.URL.Query()
    userIDStr := queryValues.Get("user_id")
    userID, err := strconv.Atoi(userIDStr)
    if err != nil {
        http.Error(w, "Invalid user ID", http.StatusBadRequest)
        return
    }

    // Query database for readings belonging to the user
    rows, err := db.Query("SELECT id, userid, timestamp, value, torquevalues, asmtimes, motionwastes, setvalue FROM readings WHERE userid = $1", userID)
    if err != nil {
        http.Error(w, "Database query error", http.StatusInternalServerError)
        return
    }
    defer rows.Close()

    var readings []Reading
    for rows.Next() {
        var reading Reading
        err := rows.Scan(&reading.ID, &reading.UserID, &reading.Timestamp, &reading.Value, pq.Array(&reading.TorqueValues), pq.Array(&reading.AsmTimes), pq.Array(&reading.MotionWastes), &reading.SetValue)

        if err != nil {
            http.Error(w, "Error scanning readings", http.StatusInternalServerError)
            return
        }
        readings = append(readings, reading)
    }

    // Send the readings as JSON
    readingsBytes, err := json.MarshalIndent(readings, "", "\t")
    if err != nil {
        http.Error(w, "Error marshaling readings", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    w.Write(readingsBytes)
}