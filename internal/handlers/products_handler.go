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

// ProductsHandler handles product list and creation
func ProductsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getAllProducts(w, r)
	case "POST":
		createProduct(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ProductHandler handles individual product operations
func ProductHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getProduct(w, r)
	case "PUT":
		updateProduct(w, r)
	case "DELETE":
		deleteProduct(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ProductBOMHandler handles BOM entries for a product
func ProductBOMHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		getProductBOM(w, r)
	case "POST":
		addProductBOMEntry(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// BOMEntryHandler handles deleting a single BOM entry
func BOMEntryHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "DELETE":
		deleteBOMEntry(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func getAllProducts(w http.ResponseWriter, r *http.Request) {
	var products []models.Product
	query := db.GetDB()

	if category := r.URL.Query().Get("category"); category != "" {
		query = query.Where("category = ?", category)
	}

	if active := r.URL.Query().Get("active"); active != "" {
		isActive := active == "true"
		query = query.Where("is_active = ?", isActive)
	}

	result := query.Find(&products)
	if result.Error != nil {
		log.Printf("Error fetching products: %v", result.Error)
		http.Error(w, "Error fetching products", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func createProduct(w http.ResponseWriter, r *http.Request) {
	var product models.Product
	if err := json.NewDecoder(r.Body).Decode(&product); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if product.Name == "" {
		http.Error(w, "Product name is required", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Create(&product)
	if result.Error != nil {
		log.Printf("Error creating product: %v", result.Error)
		http.Error(w, "Error creating product", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(product)
}

func getProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	var product models.Product
	result := db.GetDB().Preload("BOM.RawMaterial").First(&product, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Product not found", http.StatusNotFound)
		} else {
			log.Printf("Error fetching product: %v", result.Error)
			http.Error(w, "Error fetching product", http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

func updateProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	var product models.Product
	result := db.GetDB().First(&product, id)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			http.Error(w, "Product not found", http.StatusNotFound)
		} else {
			http.Error(w, "Error fetching product", http.StatusInternalServerError)
		}
		return
	}

	var updateData models.Product
	if err := json.NewDecoder(r.Body).Decode(&updateData); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	product.Name = updateData.Name
	product.SKU = updateData.SKU
	product.Description = updateData.Description
	product.Unit = updateData.Unit
	product.Category = updateData.Category
	product.IsActive = updateData.IsActive
	product.Metadata = updateData.Metadata

	result = db.GetDB().Save(&product)
	if result.Error != nil {
		log.Printf("Error updating product: %v", result.Error)
		http.Error(w, "Error updating product", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(product)
}

func deleteProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Delete(&models.Product{}, id)
	if result.Error != nil {
		log.Printf("Error deleting product: %v", result.Error)
		http.Error(w, "Error deleting product", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func getProductBOM(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	var bomEntries []models.BillOfMaterials
	result := db.GetDB().Preload("RawMaterial").Where("product_id = ?", id).Find(&bomEntries)
	if result.Error != nil {
		log.Printf("Error fetching BOM entries: %v", result.Error)
		http.Error(w, "Error fetching BOM entries", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(bomEntries)
}

func addProductBOMEntry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "Product ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid product ID", http.StatusBadRequest)
		return
	}

	var bomEntry models.BillOfMaterials
	if err := json.NewDecoder(r.Body).Decode(&bomEntry); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	bomEntry.ProductID = uint(id)

	if bomEntry.RawMaterialID == 0 {
		http.Error(w, "raw_material_id is required", http.StatusBadRequest)
		return
	}

	if bomEntry.Quantity <= 0 {
		http.Error(w, "quantity must be greater than 0", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Create(&bomEntry)
	if result.Error != nil {
		log.Printf("Error creating BOM entry: %v", result.Error)
		http.Error(w, "Error creating BOM entry", http.StatusInternalServerError)
		return
	}

	// Preload RawMaterial for the response
	db.GetDB().Preload("RawMaterial").First(&bomEntry, bomEntry.ID)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(bomEntry)
}

func deleteBOMEntry(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idStr, ok := vars["id"]
	if !ok {
		http.Error(w, "BOM entry ID required", http.StatusBadRequest)
		return
	}

	id, err := strconv.ParseUint(idStr, 10, 32)
	if err != nil {
		http.Error(w, "Invalid BOM entry ID", http.StatusBadRequest)
		return
	}

	result := db.GetDB().Delete(&models.BillOfMaterials{}, id)
	if result.Error != nil {
		log.Printf("Error deleting BOM entry: %v", result.Error)
		http.Error(w, "Error deleting BOM entry", http.StatusInternalServerError)
		return
	}

	if result.RowsAffected == 0 {
		http.Error(w, "BOM entry not found", http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
