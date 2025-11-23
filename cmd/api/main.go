package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"data-storage/internal/auth"
	"data-storage/internal/db"
	"data-storage/internal/handlers"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

func main() {
	// Load environment variables
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	// Initialize database connection
	dbConfig := db.LoadConfigFromEnv()
	_, err = db.InitDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}

	r := mux.NewRouter()

	// Public endpoints
	r.HandleFunc("/auth/login", handlers.LoginHandler).Methods("POST")

	// User authenticated endpoints
	r.HandleFunc("/auth/register-device", auth.RequireUserAuth(handlers.RegisterDeviceHandler)).Methods("POST")
	r.HandleFunc("/users", auth.RequireUserAuth(handlers.UsersHandler))
	r.HandleFunc("/users/{id}", auth.RequireUserAuth(handlers.UserHandler))
	r.HandleFunc("/devices", auth.RequireUserAuth(handlers.DevicesHandler))
	r.HandleFunc("/devices/{id}", auth.RequireUserAuth(handlers.DeviceHandler))
	
	// Signal configurations (requires user auth)
	r.HandleFunc("/signals", auth.RequireUserAuth(handlers.SignalsHandler))
	r.HandleFunc("/signals/{id}", auth.RequireUserAuth(handlers.SignalHandler))
	r.HandleFunc("/devices/{device_id}/signals", auth.RequireUserAuth(handlers.DeviceSignalsHandler)).Methods("GET")
	
	// Signal values - GET requires user auth, POST allows both user and device auth
	r.HandleFunc("/signal-values", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			auth.RequireAnyAuth(handlers.CreateSignalValue)(w, r)
		} else {
			auth.RequireUserAuth(handlers.SignalValuesHandler)(w, r)
		}
	})
	r.HandleFunc("/signal-values/{id}", auth.RequireUserAuth(handlers.SignalValueHandler))
	r.HandleFunc("/signals/{signal_id}/values", auth.RequireUserAuth(handlers.SignalValuesBySignalHandler)).Methods("GET")

	// Legacy endpoints for backward compatibility
	r.HandleFunc("/readings", auth.RequireAnyAuth(handlers.ReadingsHandler))
	r.HandleFunc("/readings/{user_id}", handlers.UserReadingsHandler)
	r.HandleFunc("/users/rfid/{rfid}", handlers.GetUserByRFIDHandler)

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		Debug:          true,
	})

	handler := c.Handler(r)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, handler))
}

