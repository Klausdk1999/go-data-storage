package handlers

import (
	"encoding/json"
	"io"
	"net/http"

	"data-storage/internal/db"

	"github.com/gorilla/mux"
)

// tableForEntity maps URL entity names to database table names.
func tableForEntity(entity string) string {
	switch entity {
	case "products":
		return "products"
	case "users":
		return "users"
	case "devices":
		return "devices"
	default:
		return ""
	}
}

// ImageUploadHandler handles PUT /{entity}/{id}/image
func ImageUploadHandler(entity string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table := tableForEntity(entity)
		if table == "" {
			http.Error(w, "invalid entity", http.StatusBadRequest)
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]

		// Limit request body to 2MB
		const maxSize = 2 * 1024 * 1024
		r.Body = http.MaxBytesReader(w, r.Body, maxSize)

		file, _, err := r.FormFile("image")
		if err != nil {
			if err.Error() == "http: request body too large" {
				http.Error(w, "file too large (max 2MB)", http.StatusBadRequest)
				return
			}
			http.Error(w, "failed to read image: "+err.Error(), http.StatusBadRequest)
			return
		}
		defer file.Close()

		data, err := io.ReadAll(file)
		if err != nil {
			if err.Error() == "http: request body too large" {
				http.Error(w, "file too large (max 2MB)", http.StatusBadRequest)
				return
			}
			http.Error(w, "failed to read image data: "+err.Error(), http.StatusInternalServerError)
			return
		}

		contentType := http.DetectContentType(data)

		result := db.DB.Table(table).Where("id = ?", id).Updates(map[string]interface{}{
			"image":      data,
			"image_type": contentType,
		})
		if result.Error != nil {
			http.Error(w, "failed to save image: "+result.Error.Error(), http.StatusInternalServerError)
			return
		}
		if result.RowsAffected == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"size":   len(data),
			"type":   contentType,
		})
	}
}

// ImageDownloadHandler handles GET /{entity}/{id}/image
func ImageDownloadHandler(entity string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table := tableForEntity(entity)
		if table == "" {
			http.Error(w, "invalid entity", http.StatusBadRequest)
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]

		var result struct {
			Image     []byte
			ImageType string
		}

		err := db.DB.Table(table).Select("image, image_type").Where("id = ?", id).Scan(&result).Error
		if err != nil {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		if len(result.Image) == 0 {
			http.Error(w, "no image", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", result.ImageType)
		w.Header().Set("Cache-Control", "public, max-age=3600")
		w.Write(result.Image)
	}
}

// ImageDeleteHandler handles DELETE /{entity}/{id}/image
func ImageDeleteHandler(entity string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		table := tableForEntity(entity)
		if table == "" {
			http.Error(w, "invalid entity", http.StatusBadRequest)
			return
		}

		vars := mux.Vars(r)
		id := vars["id"]

		result := db.DB.Table(table).Where("id = ?", id).Updates(map[string]interface{}{
			"image":      nil,
			"image_type": "",
		})
		if result.Error != nil {
			http.Error(w, "failed to delete image: "+result.Error.Error(), http.StatusInternalServerError)
			return
		}
		if result.RowsAffected == 0 {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
		})
	}
}
