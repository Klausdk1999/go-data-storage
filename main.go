package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	 _ "github.com/lib/pq"
)

type Person struct {
	Name string `json:"name"`
	Nickname string `json:"nickname"`
}

const (
	host	 = "localhost"
	port	 = 5432
	user    = "postgres"
	password = "123"
	dbname   = "data-read"
)

func main() {
	http.HandleFunc("/", GetHandler )
	http.HandleFunc("/insert", PostHandler )
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func OpenConnection() *sql.DB {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host,port,user,password,dbname)

	db, err := sql.Open("postgres", psqlInfo)
	if err != nil {
		panic(err)
	}

	return db
}

func GetHandler(w http.ResponseWriter, r *http.Request) {
	db := OpenConnection()
	
	rows, err := db.Query("SELECT * FROM person")
	if err != nil {
		log.Fatal(err)
	}

	var people []Person

	for rows.Next() {
		var person Person
		err := rows.Scan(&person.Name, &person.Nickname)
		if err != nil {
			log.Fatal(err)
		}
		people = append(people, person)
	}

	peopleBytes, _ := json.MarshalIndent(people, "", "\t")

	w.Header().Set("Content-Type", "application/json")
	w.Write(peopleBytes)

	defer rows.Close()
	defer db.Close()
}

func PostHandler(w http.ResponseWriter, r *http.Request) {
	db := OpenConnection()

	var person Person
	err := json.NewDecoder(r.Body).Decode(&person)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	sqlStatement := `INSERT INTO person (name, nickname) VALUES ($1, $2)`
	_, err = db.Exec(sqlStatement, person.Name, person.Nickname)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		panic(err)
	}

	w.WriteHeader(http.StatusOK)
	defer db.Close()
}

