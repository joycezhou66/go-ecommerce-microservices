package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/joycezhou/go-ecommerce-microservices/shared/database"
	"github.com/joycezhou/go-ecommerce-microservices/shared/middleware"
)

type Product struct {
	ID          uint      `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	Stock       int       `json:"stock"`
	Category    string    `json:"category"`
	ImageURL    string    `json:"image_url"`
	CreatedAt   time.Time `json:"created_at"`
}

type Category struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}

var db *sql.DB

func main() {
	var err error
	db, err = database.NewConnection("products_db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	initDB()

	r := mux.NewRouter()
	r.Use(middleware.CORS)

	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/products", getProducts).Methods("GET")
	r.HandleFunc("/products/{id}", getProduct).Methods("GET")
	r.HandleFunc("/products", createProduct).Methods("POST")
	r.HandleFunc("/products/{id}", updateProduct).Methods("PUT")
	r.HandleFunc("/products/{id}", deleteProduct).Methods("DELETE")
	r.HandleFunc("/products/{id}/stock", updateStock).Methods("PATCH")
	r.HandleFunc("/categories", getCategories).Methods("GET")

	log.Println("Product service running on :8002")
	log.Fatal(http.ListenAndServe(":8002", r))
}

func initDB() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS categories (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) UNIQUE NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			price DECIMAL(10,2) NOT NULL,
			stock INT DEFAULT 0,
			category VARCHAR(100),
			image_url TEXT,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			log.Fatal("Failed to create table:", err)
		}
	}

	// Insert sample categories
	categories := []string{"Electronics", "Clothing", "Books", "Home & Garden", "Sports"}
	for _, cat := range categories {
		db.Exec("INSERT INTO categories (name) VALUES ($1) ON CONFLICT DO NOTHING", cat)
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func getProducts(w http.ResponseWriter, r *http.Request) {
	category := r.URL.Query().Get("category")
	search := r.URL.Query().Get("search")
	limit := r.URL.Query().Get("limit")
	offset := r.URL.Query().Get("offset")

	if limit == "" {
		limit = "50"
	}
	if offset == "" {
		offset = "0"
	}

	query := "SELECT id, name, description, price, stock, category, image_url, created_at FROM products WHERE 1=1"
	args := []interface{}{}
	argCount := 0

	if category != "" {
		argCount++
		query += " AND category = $" + strconv.Itoa(argCount)
		args = append(args, category)
	}

	if search != "" {
		argCount++
		query += " AND (name ILIKE $" + strconv.Itoa(argCount) + " OR description ILIKE $" + strconv.Itoa(argCount) + ")"
		args = append(args, "%"+search+"%")
	}

	argCount++
	query += " ORDER BY created_at DESC LIMIT $" + strconv.Itoa(argCount)
	args = append(args, limit)

	argCount++
	query += " OFFSET $" + strconv.Itoa(argCount)
	args = append(args, offset)

	rows, err := db.Query(query, args...)
	if err != nil {
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		var p Product
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.Category, &p.ImageURL, &p.CreatedAt)
		if err != nil {
			continue
		}
		products = append(products, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(products)
}

func getProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var p Product
	err := db.QueryRow(
		"SELECT id, name, description, price, stock, category, image_url, created_at FROM products WHERE id = $1",
		id,
	).Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.Category, &p.ImageURL, &p.CreatedAt)

	if err != nil {
		http.Error(w, "Product not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(p)
}

func createProduct(w http.ResponseWriter, r *http.Request) {
	var p Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := db.QueryRow(
		`INSERT INTO products (name, description, price, stock, category, image_url)
		 VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at`,
		p.Name, p.Description, p.Price, p.Stock, p.Category, p.ImageURL,
	).Scan(&p.ID, &p.CreatedAt)

	if err != nil {
		http.Error(w, "Failed to create product", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(p)
}

func updateProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var p Product
	if err := json.NewDecoder(r.Body).Decode(&p); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(
		`UPDATE products SET name = $1, description = $2, price = $3, stock = $4, category = $5, image_url = $6 WHERE id = $7`,
		p.Name, p.Description, p.Price, p.Stock, p.Category, p.ImageURL, id,
	)

	if err != nil {
		http.Error(w, "Failed to update product", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Product updated successfully"})
}

func deleteProduct(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	_, err := db.Exec("DELETE FROM products WHERE id = $1", id)
	if err != nil {
		http.Error(w, "Failed to delete product", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func updateStock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id := vars["id"]

	var stock struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&stock); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := db.Exec("UPDATE products SET stock = stock + $1 WHERE id = $2", stock.Quantity, id)
	if err != nil {
		http.Error(w, "Failed to update stock", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Stock updated successfully"})
}

func getCategories(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query("SELECT id, name FROM categories ORDER BY name")
	if err != nil {
		http.Error(w, "Failed to fetch categories", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	categories := []Category{}
	for rows.Next() {
		var c Category
		rows.Scan(&c.ID, &c.Name)
		categories = append(categories, c)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(categories)
}
