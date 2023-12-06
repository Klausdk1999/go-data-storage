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
	TorqueValues  []float64 `json:"torque_values"`
    AsmTimes      []int    `json:"asm_times"`
    MotionWastes  []int    `json:"motion_wastes"`
    SetValue      float64   `json:"set_value"`
}