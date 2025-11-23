package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
)

var (
	host     string
	port     int
	user     string
	password string
	dbname   string
)

func init() {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found")
	}

	host = os.Getenv("DB_HOST")
	portStr := os.Getenv("DB_PORT")
	user = os.Getenv("DB_USER")
	password = os.Getenv("DB_PASSWORD")
	dbname = os.Getenv("DB_NAME")

	port, err = strconv.Atoi(portStr)
	if err != nil {
		log.Fatal("Invalid port number in environment variables")
	}
}

func main() {
	// Initialize database connection
	InitDB()

	r := mux.NewRouter()

	// Public endpoints
	r.HandleFunc("/auth/login", LoginHandler).Methods("POST")

	// User authenticated endpoints
	r.HandleFunc("/auth/register-device", RequireUserAuth(RegisterDeviceHandler)).Methods("POST")
	r.HandleFunc("/users", RequireUserAuth(UsersHandler))
	r.HandleFunc("/users/{id}", RequireUserAuth(UserHandler))
	r.HandleFunc("/devices", RequireUserAuth(DevicesHandler))
	r.HandleFunc("/devices/{id}", RequireUserAuth(DeviceHandler))
	
	// Signal configurations (requires user auth)
	r.HandleFunc("/signals", RequireUserAuth(SignalsHandler))
	r.HandleFunc("/signals/{id}", RequireUserAuth(SignalHandler))
	r.HandleFunc("/devices/{device_id}/signals", RequireUserAuth(DeviceSignalsHandler)).Methods("GET")
	
	// Signal values - GET requires user auth, POST allows both user and device auth
	r.HandleFunc("/signal-values", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			RequireAnyAuth(createSignalValue)(w, r)
		} else {
			RequireUserAuth(SignalValuesHandler)(w, r)
		}
	})
	r.HandleFunc("/signal-values/{id}", RequireUserAuth(SignalValueHandler))
	r.HandleFunc("/signals/{signal_id}/values", RequireUserAuth(SignalValuesBySignalHandler)).Methods("GET")

	// Legacy endpoints for backward compatibility
	r.HandleFunc("/readings", RequireAnyAuth(func(w http.ResponseWriter, r *http.Request) {
		// Redirect to signals
		r.URL.Path = "/signals"
		SignalsHandler(w, r)
	}))
	r.HandleFunc("/readings/{user_id}", UserReadingsHandler)
	r.HandleFunc("/users/rfid/{rfid}", GetUserByRFIDHandler)

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
		Debug:          true,
	})

	handler := c.Handler(r)

	log.Println("Server starting on :8080")
	log.Fatal(http.ListenAndServe(":8080", handler))
}
