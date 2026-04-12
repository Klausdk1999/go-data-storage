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

	// Generic device data endpoint (API key or device token auth) — must be before /devices/{id}
	r.HandleFunc("/devices/data", handlers.GenericDataHandler).Methods("POST")

	// Admin-only endpoints
	r.HandleFunc("/auth/register-device", auth.RequireAdmin(handlers.RegisterDeviceHandler)).Methods("POST")
	r.HandleFunc("/users", auth.RequireAdmin(handlers.UsersHandler))
	r.HandleFunc("/users/{id}", auth.RequireAdmin(handlers.UserHandler))
	r.HandleFunc("/users/{id}/preferences", auth.RequireUserAuth(handlers.UserPreferencesHandler)).Methods("GET", "PUT")
	r.HandleFunc("/devices", auth.RequireAdmin(handlers.DevicesHandler))
	r.HandleFunc("/devices/{id}", auth.RequireAdmin(handlers.DeviceHandler))

	// Signal configurations (admin only)
	r.HandleFunc("/signals", auth.RequireAdmin(handlers.SignalsHandler))
	r.HandleFunc("/signals/{id}", auth.RequireAdmin(handlers.SignalHandler))
	r.HandleFunc("/devices/{device_id}/signals", auth.RequireAdmin(handlers.DeviceSignalsHandler)).Methods("GET")
	
	// Signal values - GET requires user auth, POST allows both user and device auth
	r.HandleFunc("/signal-values", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" {
			auth.RequireAnyAuth(handlers.CreateSignalValue)(w, r)
		} else {
			auth.RequireAdmin(handlers.SignalValuesHandler)(w, r)
		}
	})
	r.HandleFunc("/signal-values/{id}", auth.RequireAdmin(handlers.SignalValueHandler))
	r.HandleFunc("/signals/{signal_id}/values", auth.RequireAdmin(handlers.SignalValuesBySignalHandler)).Methods("GET")

	// MES: Products (admin only)
	r.HandleFunc("/products", auth.RequireAdmin(handlers.ProductsHandler))
	r.HandleFunc("/products/{id}", auth.RequireAdmin(handlers.ProductHandler))
	r.HandleFunc("/products/{id}/bom", auth.RequireAdmin(handlers.ProductBOMHandler))

	// MES: Raw Materials (admin only)
	r.HandleFunc("/raw-materials", auth.RequireAdmin(handlers.RawMaterialsHandler))
	r.HandleFunc("/raw-materials/{id}", auth.RequireAdmin(handlers.RawMaterialHandler))
	r.HandleFunc("/raw-materials/{id}/adjust-stock", auth.RequireAdmin(handlers.AdjustStockHandler)).Methods("POST")

	// MES: Customers (admin only)
	r.HandleFunc("/customers", auth.RequireAdmin(handlers.CustomersHandler))
	r.HandleFunc("/customers/{id}", auth.RequireAdmin(handlers.CustomerByIDHandler))

	// MES: Production Orders
	r.HandleFunc("/production-orders", auth.RequireUserAuth(handlers.ProductionOrdersHandler))
	r.HandleFunc("/production-orders/{id}", auth.RequireUserAuth(handlers.ProductionOrderHandler))
	r.HandleFunc("/production-orders/{id}/status", auth.RequireAdmin(handlers.UpdateOrderStatusHandler)).Methods("PUT")
	r.HandleFunc("/production-orders/{id}/signal-values", auth.RequireAdmin(handlers.OrderSignalValuesHandler)).Methods("GET")

	// MES: Stock Movements
	r.HandleFunc("/stock-movements", auth.RequireAdmin(handlers.StockMovementsHandler)).Methods("GET")

	// MES: BOM entries
	r.HandleFunc("/bom/{id}", auth.RequireAdmin(handlers.BOMEntryHandler)).Methods("DELETE")

	// MES: Services
	r.HandleFunc("/services", auth.RequireUserAuth(handlers.ServicesHandler))
	r.HandleFunc("/services/{id}", auth.RequireUserAuth(handlers.ServiceHandler))

	// MES: Time Entries
	r.HandleFunc("/time-entries", auth.RequireUserAuth(handlers.TimeEntriesHandler))
	r.HandleFunc("/time-entries/{id}", auth.RequireUserAuth(handlers.TimeEntryHandler))

	// Legacy endpoints for backward compatibility
	r.HandleFunc("/readings", auth.RequireAnyAuth(handlers.ReadingsHandler))
	r.HandleFunc("/readings/{user_id}", handlers.UserReadingsHandler)
	r.HandleFunc("/users/rfid/{rfid}", handlers.GetUserByRFIDHandler)


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

