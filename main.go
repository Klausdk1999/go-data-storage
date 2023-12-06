package main

import (
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
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
    r := mux.NewRouter()
    
    r.HandleFunc("/users", UsersHandler)
    r.HandleFunc("/readings", ReadingsHandler)
    r.HandleFunc("/readings/{user_id}", UserReadingsHandler)
    r.HandleFunc("/users/rfid/{rfid}", GetUserByRFIDHandler)

    c := cors.New(cors.Options{
        AllowedOrigins: []string{"*"},
        AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
        AllowedHeaders: []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization"},
        Debug: true,
    })

    handler := c.Handler(r)

    log.Fatal(http.ListenAndServe(":8080", handler))
}
