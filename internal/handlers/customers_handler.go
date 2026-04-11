package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"data-storage/internal/db"
	"data-storage/internal/models"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
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

// CustomerByIDHandler handles operations on a single customer
func CustomerByIDHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getCustomerByID(w, r)
	case "PUT":
		updateCustomer(w, r)
	case "DELETE":
		deleteCustomer(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getAllCustomers(w http.ResponseWriter, r *http.Request) {
	var customers []models.Customer
	query := db.GetDB().Order("name ASC")

	if search := r.URL.Query().Get("search"); search != "" {
		query = query.Where("name ILIKE ?", "%"+search+"%")
	}

	if err := query.Find(&customers).Error; err != nil {
		log.Printf("Error fetching customers: %v", err)
		http.Error(w, "Error fetching customers", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customers)
}

func getCustomerByID(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid customer ID", http.StatusBadRequest)
		return
	}

	var customer models.Customer
	if err := db.GetDB().First(&customer, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Customer not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching customer: %v", err)
			http.Error(w, "Error fetching customer", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
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

	if customer.Phone == "" {
		http.Error(w, "phone is required", http.StatusBadRequest)
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

func updateCustomer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid customer ID", http.StatusBadRequest)
		return
	}

	var customer models.Customer
	if err := db.GetDB().First(&customer, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Customer not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching customer: %v", err)
			http.Error(w, "Error fetching customer", http.StatusInternalServerError)
		}
		return
	}

	var updates models.Customer
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Validate required fields if provided
	if updates.Name != "" {
		customer.Name = updates.Name
	}
	if updates.Phone != "" {
		customer.Phone = updates.Phone
	}
	customer.CNPJ = updates.CNPJ
	customer.Address = updates.Address

	if err := db.GetDB().Save(&customer).Error; err != nil {
		log.Printf("Error updating customer: %v", err)
		http.Error(w, "Error updating customer", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(customer)
}

func deleteCustomer(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr := vars["id"]
	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid customer ID", http.StatusBadRequest)
		return
	}

	var customer models.Customer
	if err := db.GetDB().First(&customer, id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Customer not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching customer: %v", err)
			http.Error(w, "Error fetching customer", http.StatusInternalServerError)
		}
		return
	}

	if err := db.GetDB().Delete(&customer).Error; err != nil {
		log.Printf("Error deleting customer: %v", err)
		http.Error(w, "Error deleting customer", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// findOrCreateCustomer looks up a customer by name only (no auto-create).
// Used by production orders handler when customer_name is provided.
func findOrCreateCustomer(name string) (*models.Customer, error) {
	var customer models.Customer
	result := db.GetDB().Where("name = ?", name).First(&customer)
	if result.Error == nil {
		return &customer, nil
	}
	// No auto-create: customers must be registered via the customers API first
	return nil, result.Error
}
