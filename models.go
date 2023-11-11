package main

type User struct {
    ID       int    `json:"id"`
    Name     string `json:"name"`
}

type Reading struct {
    ID        int     `json:"id"`
    UserID    int     `json:"user_id"`
    Timestamp string  `json:"timestamp"`
    Value     float64 `json:"value"`
}