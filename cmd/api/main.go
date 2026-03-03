package main

import (
	"log"
	"net/http"
	"os"

	"data-storage/internal/auth"
	"data-storage/internal/db"
	"data-storage/internal/handlers"
	"data-storage/internal/mqtt"
	"data-storage/internal/plugins/ttn"

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

	// Start embedded MQTT broker (if configured)
	brokerConfig := mqtt.LoadBrokerConfigFromEnv()
	broker, err := mqtt.StartBroker(brokerConfig)
	if err != nil {
		log.Printf("Warning: Failed to start embedded MQTT broker: %v", err)
		log.Println("Continuing without embedded MQTT broker")
	}
	if broker != nil {
		defer broker.Close()
	}

	r := mux.NewRouter()

	// Initialize TTN plugin (no-ops if TTN_ENABLED != "true")
	ttnCleanup := ttn.Register(r, auth.RequireUserAuth)
	defer ttnCleanup()

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

	// MES: Products
	r.HandleFunc("/products", auth.RequireUserAuth(handlers.ProductsHandler))
	r.HandleFunc("/products/{id}", auth.RequireUserAuth(handlers.ProductHandler))
	r.HandleFunc("/products/{id}/bom", auth.RequireUserAuth(handlers.ProductBOMHandler))

	// MES: Raw Materials
	r.HandleFunc("/raw-materials", auth.RequireUserAuth(handlers.RawMaterialsHandler))
	r.HandleFunc("/raw-materials/{id}", auth.RequireUserAuth(handlers.RawMaterialHandler))
	r.HandleFunc("/raw-materials/{id}/adjust-stock", auth.RequireUserAuth(handlers.AdjustStockHandler)).Methods("POST")

	// MES: Production Orders
	r.HandleFunc("/production-orders", auth.RequireUserAuth(handlers.ProductionOrdersHandler))
	r.HandleFunc("/production-orders/{id}", auth.RequireUserAuth(handlers.ProductionOrderHandler))
	r.HandleFunc("/production-orders/{id}/status", auth.RequireUserAuth(handlers.UpdateOrderStatusHandler)).Methods("PUT")
	r.HandleFunc("/production-orders/{id}/signal-values", auth.RequireUserAuth(handlers.OrderSignalValuesHandler)).Methods("GET")

	// MES: Stock Movements
	r.HandleFunc("/stock-movements", auth.RequireUserAuth(handlers.StockMovementsHandler)).Methods("GET")

	// MES: BOM entries
	r.HandleFunc("/bom/{id}", auth.RequireUserAuth(handlers.BOMEntryHandler)).Methods("DELETE")

	// Legacy endpoints for backward compatibility
	r.HandleFunc("/readings", auth.RequireAnyAuth(handlers.ReadingsHandler))
	r.HandleFunc("/readings/{user_id}", handlers.UserReadingsHandler)
	r.HandleFunc("/users/rfid/{rfid}", handlers.GetUserByRFIDHandler)

	// Generic device data endpoint (API key or device token auth)
	r.HandleFunc("/devices/data", handlers.GenericDataHandler).Methods("POST")

	c := cors.New(cors.Options{
		AllowedOrigins: []string{"*"},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Accept", "Content-Type", "Content-Length", "Accept-Encoding", "X-CSRF-Token", "Authorization", "X-API-Key"},
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

