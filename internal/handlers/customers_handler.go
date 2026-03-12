package handlers

import (
	"encoding/json"
	"log"
	"net/http"

	"data-storage/internal/db"
	"data-storage/internal/models"
)

// CustomersHandler handles customer list and creation
func CustomersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllCustomers(w, r)
	case "POST":
		createCustomer(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getAllCustomers(w http.ResponseWriter, r *http.Request) {
	var customers []models.Customer
	query := db.GetDB().Order("name ASC")

	if search := r.URL.Query().Get("search"); search != "" {
		query = query.Where("name LIKE ?", "%"+search+"%")
	}

	if err := query.Find(&customers).Error; err != nil {
		log.Printf("Error fetching customers: %v", err)
		http.Error(w, "Error fetching customers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customers)
}

func createCustomer(w http.ResponseWriter, r *http.Request) {
	var customer models.Customer
	if err := json.NewDecoder(r.Body).Decode(&customer); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if customer.Name == "" {
		http.Error(w, "name is required", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Create(&customer)
	if result.Error != nil {
		log.Printf("Error creating customer: %v", result.Error)
		http.Error(w, "Error creating customer", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(customer)
}

// findOrCreateCustomer looks up a customer by name, creating one if it doesn't exist.
func findOrCreateCustomer(name string) (*models.Customer, error) {
	var customer models.Customer
	result := db.GetDB().Where("name = ?", name).First(&customer)
	if result.Error == nil {
		return &customer, nil
	}

	customer = models.Customer{Name: name}
	if err := db.GetDB().Create(&customer).Error; err != nil {
		return nil, err
	}
	return &customer, nil
}
