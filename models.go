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

type TorqueData struct {
	Routine       int       `json:"[RN_001_30_3]"`
    TorqueValues  []float32 `json:"torque_values"`
    AsmTimes      []uint    `json:"asm_times"`
    MotionWastes  []uint    `json:"motion_wastes"`
    SetValue      float32   `json:"set_value"`
}