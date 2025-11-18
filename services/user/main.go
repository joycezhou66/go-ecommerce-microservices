package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	"github.com/joycezhou/go-ecommerce-microservices/shared/database"
	"github.com/joycezhou/go-ecommerce-microservices/shared/middleware"
	"golang.org/x/crypto/bcrypt"
)

type User struct {
	ID        uint      `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"password,omitempty"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	Phone     string    `json:"phone"`
	Address   string    `json:"address"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

var db *sql.DB

func main() {
	var err error
	db, err = database.NewConnection("users_db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	initDB()

	r := mux.NewRouter()
	r.Use(middleware.CORS)

	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/register", register).Methods("POST")
	r.HandleFunc("/login", login).Methods("POST")
	r.HandleFunc("/users/{id}", getUser).Methods("GET")
	r.HandleFunc("/users/{id}", updateUser).Methods("PUT")

	log.Println("User service running on :8001")
	log.Fatal(http.ListenAndServe(":8001", r))
}

func initDB() {
	query := `
	CREATE TABLE IF NOT EXISTS users (
		id SERIAL PRIMARY KEY,
		email VARCHAR(255) UNIQUE NOT NULL,
		password VARCHAR(255) NOT NULL,
		first_name VARCHAR(100),
		last_name VARCHAR(100),
		phone VARCHAR(20),
		address TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Failed to create users table:", err)
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func register(w http.ResponseWriter, r *http.Request) {
	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		http.Error(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	err = db.QueryRow(
		`INSERT INTO users (email, password, first_name, last_name, phone, address)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		user.Email, string(hashedPassword), user.FirstName, user.LastName, user.Phone, user.Address,
	).Scan(&user.ID, &user.CreatedAt)

	if err != nil {
		http.Error(w, "Email already exists", http.StatusConflict)
		return
	}

	token, err := generateToken(user.ID, user.Email)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	user.Password = ""
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{Token: token, User: user})
}

func login(w http.ResponseWriter, r *http.Request) {
	var credentials struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&credentials); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var user User
	var hashedPassword string
	err := db.QueryRow(
		`SELECT id, email, password, first_name, last_name, phone, address, created_at
		 FROM users WHERE email = $1`,
		credentials.Email,
	).Scan(&user.ID, &user.Email, &hashedPassword, &user.FirstName, &user.LastName, &user.Phone, &user.Address, &user.CreatedAt)

	if err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(credentials.Password)); err != nil {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := generateToken(user.ID, user.Email)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{Token: token, User: user})
}

func getUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var user User
	err = db.QueryRow(
		`SELECT id, email, first_name, last_name, phone, address, created_at
		 FROM users WHERE id = $1`,
		id,
	).Scan(&user.ID, &user.Email, &user.FirstName, &user.LastName, &user.Phone, &user.Address, &user.CreatedAt)

	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(user)
}

func updateUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		http.Error(w, "Invalid user ID", http.StatusBadRequest)
		return
	}

	var user User
	if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err = db.Exec(
		`UPDATE users SET first_name = $1, last_name = $2, phone = $3, address = $4 WHERE id = $5`,
		user.FirstName, user.LastName, user.Phone, user.Address, id,
	)

	if err != nil {
		http.Error(w, "Failed to update user", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "User updated successfully"})
}

func generateToken(userID uint, email string) (string, error) {
	claims := &middleware.Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(middleware.GetJWTSecret())
}
