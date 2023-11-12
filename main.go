package main

import (
	"log"
	"net/http"
	"github.com/rs/cors"

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
    mux := http.NewServeMux()
    
    // Register your handlers
    mux.HandleFunc("/users", UsersHandler)
    mux.HandleFunc("/readings", ReadingsHandler)
    mux.HandleFunc("/user_readings", UserReadingsHandler)

    // Setup CORS
    c := cors.New(cors.Options{
        AllowedOrigins: []string{"http://localhost:3000"}, // Adjust this to your frontend's URL
        AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders: []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
        // Enable Debugging for testing, consider disabling in production
        Debug: true,
    })

    // Apply CORS middleware
    handler := c.Handler(mux)

    // Start server with CORS handler
    log.Fatal(http.ListenAndServe(":8080", handler))
}
