package main

import (
	"log"
	"net/http"

	"github.com/rs/cors"
    "github.com/gorilla/mux"
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
    r := mux.NewRouter()
    
    // Register your handlers
    r.HandleFunc("/users", UsersHandler)
    r.HandleFunc("/readings", ReadingsHandler)
    r.HandleFunc("/readings/{user_id}", UserReadingsHandler)

    // Setup CORS
    c := cors.New(cors.Options{
        AllowedOrigins: []string{"*"}, // Allow all origins for testing
        AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders: []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
        Debug: true, // Enable Debugging for testing, consider disabling in production
    })

    // Apply CORS middleware
    handler := c.Handler(r)

    // Start server with CORS handler
    log.Fatal(http.ListenAndServe(":8080", handler))
}
