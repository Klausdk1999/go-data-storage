package main

import (
	"encoding/json"
	"html/template"
	"net/http"

	_ "github.com/lib/pq"
)

var globalTorqueData TorqueData

func receiveTorqueData(w http.ResponseWriter, r *http.Request) {
    var data TorqueData
    err := json.NewDecoder(r.Body).Decode(&data)
    if err != nil {
        http.Error(w, err.Error(), http.StatusBadRequest)
        return
    }

    globalTorqueData = data
    w.WriteHeader(http.StatusOK)
}

func serveTorquePage(w http.ResponseWriter, r *http.Request) {
    tmpl := template.Must(template.ParseFiles("template.html"))
    tmpl.Execute(w, globalTorqueData)
}