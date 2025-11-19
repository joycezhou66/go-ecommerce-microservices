package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

var db *sql.DB

// Models
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

type CartItem struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"user_id"`
	ProductID uint      `json:"product_id"`
	Quantity  int       `json:"quantity"`
	Price     float64   `json:"price"`
	Name      string    `json:"name"`
	ImageURL  string    `json:"image_url"`
	CreatedAt time.Time `json:"created_at"`
}

type Cart struct {
	Items      []CartItem `json:"items"`
	TotalItems int        `json:"total_items"`
	TotalPrice float64    `json:"total_price"`
}

type Order struct {
	ID            uint        `json:"id"`
	UserID        uint        `json:"user_id"`
	Status        string      `json:"status"`
	TotalAmount   float64     `json:"total_amount"`
	ShippingAddr  string      `json:"shipping_address"`
	PaymentMethod string      `json:"payment_method"`
	PaymentStatus string      `json:"payment_status"`
	Items         []OrderItem `json:"items,omitempty"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

type OrderItem struct {
	ID        uint    `json:"id"`
	OrderID   uint    `json:"order_id"`
	ProductID uint    `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type AuthResponse struct {
	Token string `json:"token"`
	User  User   `json:"user"`
}

type Claims struct {
	UserID uint   `json:"user_id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

var jwtSecret = []byte("default-secret-key-change-in-production")

func main() {
	// Connect to database
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable required")
	}

	var err error
	db, err = sql.Open("postgres", databaseURL)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatal("Failed to ping database:", err)
	}

	log.Println("Connected to database")

	// Set up router
	r := mux.NewRouter()
	r.Use(corsMiddleware)

	// Health check
	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/api/health", healthCheck).Methods("GET")

	// Auth routes
	r.HandleFunc("/api/register", register).Methods("POST")
	r.HandleFunc("/api/login", login).Methods("POST")

	// Product routes
	r.HandleFunc("/api/products", getProducts).Methods("GET")
	r.HandleFunc("/api/products/{id}", getProduct).Methods("GET")
	r.HandleFunc("/api/categories", getCategories).Methods("GET")

	// Cart routes
	r.HandleFunc("/api/cart/{user_id}", getCart).Methods("GET")
	r.HandleFunc("/api/cart/{user_id}/items", addToCart).Methods("POST")
	r.HandleFunc("/api/cart/{user_id}/items/{item_id}", updateCartItem).Methods("PUT")
	r.HandleFunc("/api/cart/{user_id}/items/{item_id}", removeFromCart).Methods("DELETE")
	r.HandleFunc("/api/cart/{user_id}", clearCart).Methods("DELETE")

	// Order routes
	r.HandleFunc("/api/orders", createOrder).Methods("POST")
	r.HandleFunc("/api/orders/user/{user_id}", getOrdersByUser).Methods("GET")
	r.HandleFunc("/api/orders/{id}", getOrder).Methods("GET")

	// Serve static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./frontend/static"))))
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./frontend/templates")))

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server running on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

// Auth handlers
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

	token, _ := generateToken(user.ID, user.Email)
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

	token, _ := generateToken(user.ID, user.Email)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(AuthResponse{Token: token, User: user})
}

func generateToken(userID uint, email string) (string, error) {
	claims := &Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// Product handlers
func getProducts(w http.ResponseWriter, r *http.Request) {
	rows, err := db.Query(
		`SELECT id, name, description, price, stock, category, image_url, created_at
		 FROM products ORDER BY created_at DESC`,
	)
	if err != nil {
		log.Printf("Error fetching products: %v", err)
		http.Error(w, "Failed to fetch products", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	products := []Product{}
	for rows.Next() {
		var p Product
		err := rows.Scan(&p.ID, &p.Name, &p.Description, &p.Price, &p.Stock, &p.Category, &p.ImageURL, &p.CreatedAt)
		if err != nil {
			log.Printf("Error scanning product: %v", err)
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

// Cart handlers
func getCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	rows, err := db.Query(
		`SELECT id, user_id, product_id, quantity, price, name, image_url, created_at
		 FROM cart_items WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		http.Error(w, "Failed to fetch cart", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	cart := Cart{Items: []CartItem{}}
	for rows.Next() {
		var item CartItem
		err := rows.Scan(&item.ID, &item.UserID, &item.ProductID, &item.Quantity, &item.Price, &item.Name, &item.ImageURL, &item.CreatedAt)
		if err != nil {
			continue
		}
		cart.Items = append(cart.Items, item)
		cart.TotalItems += item.Quantity
		cart.TotalPrice += item.Price * float64(item.Quantity)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(cart)
}

func addToCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	var item CartItem
	if err := json.NewDecoder(r.Body).Decode(&item); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(
		`INSERT INTO cart_items (user_id, product_id, quantity, price, name, image_url)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (user_id, product_id) DO UPDATE SET quantity = cart_items.quantity + $3`,
		userID, item.ProductID, item.Quantity, item.Price, item.Name, item.ImageURL,
	)

	if err != nil {
		http.Error(w, "Failed to add item to cart", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Item added to cart"})
}

func updateCartItem(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]
	itemID := vars["item_id"]

	var update struct {
		Quantity int `json:"quantity"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if update.Quantity <= 0 {
		db.Exec("DELETE FROM cart_items WHERE id = $1 AND user_id = $2", itemID, userID)
	} else {
		db.Exec("UPDATE cart_items SET quantity = $1 WHERE id = $2 AND user_id = $3", update.Quantity, itemID, userID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Cart updated"})
}

func removeFromCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]
	itemID := vars["item_id"]

	db.Exec("DELETE FROM cart_items WHERE id = $1 AND user_id = $2", itemID, userID)
	w.WriteHeader(http.StatusNoContent)
}

func clearCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	db.Exec("DELETE FROM cart_items WHERE user_id = $1", userID)
	w.WriteHeader(http.StatusNoContent)
}

// Order handlers
func createOrder(w http.ResponseWriter, r *http.Request) {
	var order Order
	if err := json.NewDecoder(r.Body).Decode(&order); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	tx, err := db.Begin()
	if err != nil {
		http.Error(w, "Failed to start transaction", http.StatusInternalServerError)
		return
	}
	defer tx.Rollback()

	err = tx.QueryRow(
		`INSERT INTO orders (user_id, total_amount, shipping_address, payment_method, status, payment_status)
		 VALUES ($1, $2, $3, $4, 'pending', 'pending') RETURNING id, created_at, updated_at`,
		order.UserID, order.TotalAmount, order.ShippingAddr, order.PaymentMethod,
	).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		http.Error(w, "Failed to create order", http.StatusInternalServerError)
		return
	}

	for _, item := range order.Items {
		_, err = tx.Exec(
			`INSERT INTO order_items (order_id, product_id, name, quantity, price)
			 VALUES ($1, $2, $3, $4, $5)`,
			order.ID, item.ProductID, item.Name, item.Quantity, item.Price,
		)
		if err != nil {
			http.Error(w, "Failed to create order items", http.StatusInternalServerError)
			return
		}
	}

	// Clear cart after order
	tx.Exec("DELETE FROM cart_items WHERE user_id = $1", order.UserID)

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	order.Status = "confirmed"
	order.PaymentStatus = "completed"

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(order)
}

func getOrdersByUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	rows, err := db.Query(
		`SELECT id, user_id, status, total_amount, shipping_address, payment_method, payment_status, created_at, updated_at
		 FROM orders WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		http.Error(w, "Failed to fetch orders", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	orders := []Order{}
	for rows.Next() {
		var o Order
		rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalAmount, &o.ShippingAddr, &o.PaymentMethod, &o.PaymentStatus, &o.CreatedAt, &o.UpdatedAt)
		orders = append(orders, o)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(orders)
}

func getOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["id"]

	var order Order
	err := db.QueryRow(
		`SELECT id, user_id, status, total_amount, shipping_address, payment_method, payment_status, created_at, updated_at
		 FROM orders WHERE id = $1`,
		orderID,
	).Scan(&order.ID, &order.UserID, &order.Status, &order.TotalAmount, &order.ShippingAddr, &order.PaymentMethod, &order.PaymentStatus, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		http.Error(w, "Order not found", http.StatusNotFound)
		return
	}

	rows, _ := db.Query("SELECT id, order_id, product_id, name, quantity, price FROM order_items WHERE order_id = $1", orderID)
	if rows != nil {
		defer rows.Close()
		for rows.Next() {
			var item OrderItem
			rows.Scan(&item.ID, &item.OrderID, &item.ProductID, &item.Name, &item.Quantity, &item.Price)
			order.Items = append(order.Items, item)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(order)
}
