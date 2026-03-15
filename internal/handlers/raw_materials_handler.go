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

// RawMaterialsHandler handles raw material list and creation
func RawMaterialsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllRawMaterials(w, r)
	case "POST":
		createRawMaterial(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// RawMaterialHandler handles individual raw material operations
func RawMaterialHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getRawMaterial(w, r)
	case "PUT":
		updateRawMaterial(w, r)
	case "DELETE":
		deleteRawMaterial(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// AdjustStockHandler handles stock adjustments for a raw material
func AdjustStockHandler(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Raw material ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid raw material ID", http.StatusBadRequest)
		return
	}

	var req struct {
		Quantity     float64 `json:"quantity"`
		MovementType string  `json:"movement_type"`
		Notes        string  `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.MovementType == "" {
		req.MovementType = "adjustment"
	}

	tx := db.GetDB().Begin()

	var material models.RawMaterial
	if err := tx.First(&material, id).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			http.Error(w, "Raw material not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching raw material: %v", err)
			http.Error(w, "Error fetching raw material", http.StatusInternalServerError)
		}
		return
	}

	var newQty float64

	switch req.MovementType {
		case "in":
			// Entrada: soma ao estoque atual
			newQty = material.StockQuantity + req.Quantity

		case "out":
			// Saída: subtrai do estoque atual
			newQty = material.StockQuantity - req.Quantity
			if newQty < 0 {
				tx.Rollback()
				http.Error(w, "Stock would go negative", http.StatusBadRequest)
				return
			}

		case "adjustment":
			// Ajuste: seta diretamente para o valor informado
			newQty = req.Quantity
			if newQty < 0 {
				tx.Rollback()
				http.Error(w, "Stock quantity cannot be negative", http.StatusBadRequest)
				return
			}

		default:
			tx.Rollback()
			http.Error(w, "Invalid movement_type. Must be: in, out or adjustment", http.StatusBadRequest)
			return
	}

	material.StockQuantity = newQty
	if err := tx.Save(&material).Error; err != nil {
		tx.Rollback()
		log.Printf("Error updating stock: %v", err)
		http.Error(w, "Error updating stock", http.StatusInternalServerError)
		return
	}

	movement := models.StockMovement{
		RawMaterialID: uint(id),
		MovementType:  req.MovementType,
		Quantity:      req.Quantity,
		Notes:         req.Notes,
	}
	if err := tx.Create(&movement).Error; err != nil {
		tx.Rollback()
		log.Printf("Error creating stock movement: %v", err)
		http.Error(w, "Error creating stock movement", http.StatusInternalServerError)
		return
	}

	if err := tx.Commit().Error; err != nil {
		log.Printf("Error committing transaction: %v", err)
		http.Error(w, "Error committing transaction", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(material)
}

// StockMovementsHandler handles listing stock movements
func StockMovementsHandler(w http.ResponseWriter, r *http.Request) {
	var movements []models.StockMovement
	query := db.GetDB().Preload("RawMaterial").Order("created_at DESC")

	if rawMaterialID := r.URL.Query().Get("raw_material_id"); rawMaterialID != "" {
		query = query.Where("raw_material_id = ?", rawMaterialID)
	}

	if productionOrderID := r.URL.Query().Get("production_order_id"); productionOrderID != "" {
		query = query.Where("production_order_id = ?", productionOrderID)
	}

	limit := 100
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}
	query = query.Limit(limit)

	result := query.Find(&movements)
	if result.Error != nil {
		log.Printf("Error fetching stock movements: %v", result.Error)
		http.Error(w, "Error fetching stock movements", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(movements)
}

func getAllRawMaterials(w http.ResponseWriter, r *http.Request) {
	var materials []models.RawMaterial
	query := db.GetDB()

	if category := r.URL.Query().Get("category"); category != "" {
		query = query.Where("category = ?", category)
	}

	if active := r.URL.Query().Get("active"); active != "" {
		isActive := active == "true"
		query = query.Where("is_active = ?", isActive)
	}

	result := query.Find(&materials)
	if result.Error != nil {
		log.Printf("Error fetching raw materials: %v", result.Error)
		http.Error(w, "Error fetching raw materials", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(materials)
}

func createRawMaterial(w http.ResponseWriter, r *http.Request) {
	var material models.RawMaterial
	if err := json.NewDecoder(r.Body).Decode(&material); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if material.Name == "" {
		http.Error(w, "Raw material name is required", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Create(&material)
	if result.Error != nil {
		log.Printf("Error creating raw material: %v", result.Error)
		http.Error(w, "Error creating raw material", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(material)
}

func getRawMaterial(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Raw material ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid raw material ID", http.StatusBadRequest)
		return
	}

	var material models.RawMaterial
	result := db.GetDB().First(&material, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Raw material not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching raw material: %v", result.Error)
			http.Error(w, "Error fetching raw material", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(material)
}

func updateRawMaterial(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Raw material ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid raw material ID", http.StatusBadRequest)
		return
	}

	var material models.RawMaterial
	result := db.GetDB().First(&material, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Raw material not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching raw material", http.StatusInternalServerError)
		}
		return
	}

	var updateData models.RawMaterial
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Update fields but NOT stock_quantity (that uses adjust-stock endpoint)
	material.Name = updateData.Name
	material.SKU = updateData.SKU
	material.Description = updateData.Description
	material.Unit = updateData.Unit
	material.MinStock = updateData.MinStock
	material.Category = updateData.Category
	material.IsActive = updateData.IsActive
	material.Metadata = updateData.Metadata

	result = db.GetDB().Save(&material)
	if result.Error != nil {
		log.Printf("Error updating raw material: %v", result.Error)
		http.Error(w, "Error updating raw material", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(material)
}

func deleteRawMaterial(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Raw material ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid raw material ID", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Delete(&models.RawMaterial{}, id)
	if result.Error != nil {
		log.Printf("Error deleting raw material: %v", result.Error)
		http.Error(w, "Error deleting raw material", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "Raw material not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
