# go-data-storage

Go project to store data readings received via post request on a PostgreSQL database

## Requirements:

- Go installed on the host
- PostgreSQL database setup with:
  ` CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    rfid VARCHAR(255) NOT NULL,
    matricula VARCHAR(255) NOT NULL,
    categoria VARCHAR(255) NOT NULL
);
CREATE TABLE readings (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL,
    timestamp TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    value NUMERIC NOT NULL,
    FOREIGN KEY (user_id) REFERENCES Users(id)
);`

## Get started:

- Change the database connection parameters on main.go to be your database parameters;
- Run `go install` - installs dependencies and build;
- Run `data-storage` - starts the project;

## Endpoints:

### User

    - POST /users: Create a new user.
        Body: {"name": "User Name"}
        Responses: 200 OK, 400 Bad Request, 500 Internal Server Error

    - GET /users: Retrieve all users.
        Responses: 200 OK, 500 Internal Server Error

### Reading

    - POST /readings: Create a new reading.
        Body: {"user_id": 1, "value": 42.5}
        Responses: 200 OK, 400 Bad Request, 500 Internal Server Error

    - GET /readings: Retrieve all readings.
        Responses: 200 OK, 500 Internal Server Error

    - GET /readings/{user_id}: Retrieve readings for a specific user.
        URL Parameter: user_id (integer)
        Responses: 200 OK, 400 Bad Request, 500 Internal Server Error
