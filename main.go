package main

import (
	"log"
	"net/http"

	_ "github.com/lib/pq"
)

// Database connection parameters
const (
	host	 = "localhost"
	port	 = 5432
	user    = "postgres"
	password = "123"
	dbname   = "data-read"
)

func main() {
    http.HandleFunc("/users", UsersHandler)
    http.HandleFunc("/readings", ReadingsHandler)
    http.HandleFunc("/user_readings", UserReadingsHandler)
    log.Fatal(http.ListenAndServe(":8080", nil))
}
