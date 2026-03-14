package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"data-storage/internal/db"
	"data-storage/internal/models"

	"github.com/gorilla/mux"
	"gorm.io/gorm"
)

var validStatuses = map[string]bool{
	"planned":     true,
	"in_progress": true,
	"completed":   true,
	"cancelled":   true,
}

// ProductionOrdersHandler handles production order list and creation
func ProductionOrdersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllProductionOrders(w, r)
	case "POST":
		if r.Header.Get("X-User-Role") != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		createProductionOrder(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ProductionOrderHandler handles individual production order operations
func ProductionOrderHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getProductionOrder(w, r)
	case "PUT":
		if r.Header.Get("X-User-Role") != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		updateProductionOrder(w, r)
	case "DELETE":
		if r.Header.Get("X-User-Role") != "admin" {
			http.Error(w, "Admin access required", http.StatusForbidden)
			return
		}
		deleteProductionOrder(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// UpdateOrderStatusHandler handles status transitions for a production order
func UpdateOrderStatusHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Production order ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid production order ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if !validStatuses[req.Status] {
		http.Error(w, "Invalid status. Must be one of: planned, in_progress, completed, cancelled", http.StatusBadRequest)
		return
	}

	tx := db.GetDB().Begin()

	var order models.ProductionOrder
	if err := tx.Preload("Product.BOM.RawMaterial").First(&order, id).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Production order not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching production order: %v", err)
			http.Error(w, "Error fetching production order", http.StatusInternalServerError)
		}
		return
	}

	// Validate status transitions
	now := time.Now()
	switch {
	case order.Status == "planned" && req.Status == "in_progress":
		order.StartedAt = &now

	case order.Status == "in_progress" && req.Status == "completed":
		order.CompletedAt = &now
		// Decrement stock via BOM
		if err := decrementStock(tx, &order); err != nil {
			tx.Rollback()
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

	case req.Status == "cancelled" && order.Status != "completed":
		// Allow cancellation from any state except completed

	default:
		tx.Rollback()
		http.Error(w, fmt.Sprintf("Invalid status transition from '%s' to '%s'", order.Status, req.Status), http.StatusBadRequest)
		return
	}

	order.Status = req.Status

	if err := tx.Save(&order).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating production order status: %v", err)
		http.Error(w, "Error updating production order status", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		http.Error(w, "Error committing transaction", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func decrementStock(tx *gorm.DB, order *models.ProductionOrder) error {
	if order.Product == nil || len(order.Product.BOM) == 0 {
		return nil
	}

	for _, bom := range order.Product.BOM {
		requiredQty := bom.Quantity * order.Quantity

		var material models.RawMaterial
		if err := tx.First(&material, bom.RawMaterialID).Error; err != nil {
			return fmt.Errorf("failed to load raw material %d: %v", bom.RawMaterialID, err)
		}

		if material.StockQuantity < requiredQty {
			return fmt.Errorf("insufficient stock for material '%s' (ID %d): need %.2f, have %.2f",
				material.Name, material.ID, requiredQty, material.StockQuantity)
		}

		material.StockQuantity -= requiredQty
		if err := tx.Save(&material).Error; err != nil {
			return fmt.Errorf("failed to update stock for material %d: %v", material.ID, err)
		}

		movement := models.StockMovement{
			RawMaterialID:     material.ID,
			ProductionOrderID: &order.ID,
			MovementType:      "out",
			Quantity:          -requiredQty,
			Notes:             fmt.Sprintf("Consumed for production order #%d", order.ID),
		}
		if err := tx.Create(&movement).Error; err != nil {
			return fmt.Errorf("failed to create stock movement for material %d: %v", material.ID, err)
		}
	}

	return nil
}

func getAllProductionOrders(w http.ResponseWriter, r *http.Request) {
	var orders []models.ProductionOrder
	query := db.GetDB().Preload("Product").Preload("Device").Preload("Customer")

	if status := r.URL.Query().Get("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if productID := r.URL.Query().Get("product_id"); productID != "" {
		query = query.Where("product_id = ?", productID)
	}

	if deviceID := r.URL.Query().Get("device_id"); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}

	if customerID := r.URL.Query().Get("customer_id"); customerID != "" {
		query = query.Where("customer_id = ?", customerID)
	}

	query = query.Order("priority DESC, created_at DESC")

	result := query.Find(&orders)
	if result.Error != nil {
		log.Printf("Error fetching production orders: %v", result.Error)
		http.Error(w, "Error fetching production orders", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func createProductionOrder(w http.ResponseWriter, r *http.Request) {
	var req struct {
		models.ProductionOrder
		CustomerName string `json:"customer_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	order := req.ProductionOrder

	if order.ProductID == 0 {
		http.Error(w, "product_id is required", http.StatusBadRequest)
		return
	}

	if order.Quantity <= 0 {
		http.Error(w, "quantity must be greater than 0", http.StatusBadRequest)
		return
	}

	// Verify product exists
	var product models.Product
	if err := db.GetDB().First(&product, order.ProductID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Product not found", http.StatusBadRequest)
		} else {
			log.Printf("Error verifying product: %v", err)
			http.Error(w, "Error verifying product", http.StatusInternalServerError)
		}
		return
	}

	// Find or create customer by name
	if req.CustomerName != "" {
		customer, err := findOrCreateCustomer(req.CustomerName)
		if err != nil {
			log.Printf("Error finding/creating customer: %v", err)
			http.Error(w, "Error processing customer", http.StatusInternalServerError)
			return
		}
		order.CustomerID = &customer.ID
	}

	// Default status
	if order.Status == "" {
		order.Status = "planned"
	}

	if !validStatuses[order.Status] {
		http.Error(w, "Invalid status. Must be one of: planned, in_progress, completed, cancelled", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Create(&order)
	if result.Error != nil {
		log.Printf("Error creating production order: %v", result.Error)
		http.Error(w, "Error creating production order", http.StatusInternalServerError)
		return
	}

	// Reload with associations
	db.GetDB().Preload("Product").Preload("Device").Preload("Customer").First(&order, order.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func getProductionOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Production order ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid production order ID", http.StatusBadRequest)
		return
	}

	var order models.ProductionOrder
	result := db.GetDB().Preload("Product.BOM.RawMaterial").Preload("Device").Preload("Customer").First(&order, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Production order not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching production order: %v", result.Error)
			http.Error(w, "Error fetching production order", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func updateProductionOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Production order ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid production order ID", http.StatusBadRequest)
		return
	}

	var order models.ProductionOrder
	result := db.GetDB().First(&order, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Production order not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching production order", http.StatusInternalServerError)
		}
		return
	}

	var updateReq struct {
		models.ProductionOrder
		CustomerName *string `json:"customer_name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update fields but NOT status (that uses the status endpoint)
	order.Quantity = updateReq.Quantity
	order.Priority = updateReq.Priority
	order.DeviceID = updateReq.DeviceID
	order.WorkInstructions = updateReq.WorkInstructions
	order.QualityNotes = updateReq.QualityNotes
	order.StartedAt = updateReq.StartedAt
	order.CompletedAt = updateReq.CompletedAt
	order.PlannedDeliveryDate = updateReq.PlannedDeliveryDate
	order.Metadata = updateReq.Metadata

	// Handle customer: find or create by name
	if updateReq.CustomerName != nil {
		if *updateReq.CustomerName == "" {
			order.CustomerID = nil
		} else {
			customer, err := findOrCreateCustomer(*updateReq.CustomerName)
			if err != nil {
				log.Printf("Error finding/creating customer: %v", err)
				http.Error(w, "Error processing customer", http.StatusInternalServerError)
				return
			}
			order.CustomerID = &customer.ID
		}
	}

	result = db.GetDB().Save(&order)
	if result.Error != nil {
		log.Printf("Error updating production order: %v", result.Error)
		http.Error(w, "Error updating production order", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}

func deleteProductionOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Production order ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid production order ID", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Delete(&models.ProductionOrder{}, id)
	if result.Error != nil {
		log.Printf("Error deleting production order: %v", result.Error)
		http.Error(w, "Error deleting production order", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "Production order not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// OrderSignalValuesHandler returns signal values for the device linked to an order,
// filtered by the order's active time range
func OrderSignalValuesHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vars := mux.Vars(r)
	id, err := strconv.ParseUint(vars["id"], 10, 32)
	if err != nil {
		http.Error(w, "Invalid order ID", http.StatusBadRequest)
		return
	}

	var order models.ProductionOrder
	if db.GetDB().First(&order, id).Error != nil {
		http.Error(w, "Production order not found", http.StatusNotFound)
		return
	}

	if order.DeviceID == nil {
		http.Error(w, "Order has no linked device", http.StatusBadRequest)
		return
	}

	// Get signals for the device
	var signals []models.Signal
	db.GetDB().Where("device_id = ?", *order.DeviceID).Find(&signals)

	signalIDs := make([]uint, len(signals))
	for i, s := range signals {
		signalIDs[i] = s.ID
	}

	// Query signal values within the order's time range
	query := db.GetDB().Where("signal_id IN ?", signalIDs).Preload("Signal").Order("timestamp ASC")

	if order.StartedAt != nil {
		query = query.Where("timestamp >= ?", *order.StartedAt)
	}
	if order.CompletedAt != nil {
		query = query.Where("timestamp <= ?", *order.CompletedAt)
	}

	var values []models.SignalValue
	query.Find(&values)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(values)
}
