package main

type User struct {
	ID        int    `json:"id,omitempty"`
	Name      string `json:"name"`
	Categoria string `json:"categoria"`
	Matricula string `json:"matricula"`
	Rfid      string `json:"rfid"`
}

type Reading struct {
	ID        int     `json:"id,omitempty"`
	UserID    int     `json:"user_id"`
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}