package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/joycezhou/go-ecommerce-microservices/shared/database"
	"github.com/joycezhou/go-ecommerce-microservices/shared/middleware"
)

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

var db *sql.DB

func main() {
	var err error
	db, err = database.NewConnection("orders_db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	initDB()

	r := mux.NewRouter()
	r.Use(middleware.CORS)

	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/orders", createOrder).Methods("POST")
	r.HandleFunc("/orders/user/{user_id}", getOrdersByUser).Methods("GET")
	r.HandleFunc("/orders/{id}", getOrder).Methods("GET")
	r.HandleFunc("/orders/{id}/status", updateOrderStatus).Methods("PATCH")
	r.HandleFunc("/orders/{id}/payment", updatePaymentStatus).Methods("PATCH")

	log.Println("Order service running on :8004")
	log.Fatal(http.ListenAndServe(":8004", r))
}

func initDB() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INT NOT NULL,
			status VARCHAR(50) DEFAULT 'pending',
			total_amount DECIMAL(10,2) NOT NULL,
			shipping_address TEXT,
			payment_method VARCHAR(50),
			payment_status VARCHAR(50) DEFAULT 'pending',
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id SERIAL PRIMARY KEY,
			order_id INT NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
			product_id INT NOT NULL,
			name VARCHAR(255) NOT NULL,
			quantity INT NOT NULL,
			price DECIMAL(10,2) NOT NULL
		)`,
	}

	for _, query := range queries {
		_, err := db.Exec(query)
		if err != nil {
			log.Fatal("Failed to create table:", err)
		}
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

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

	for i := range order.Items {
		_, err = tx.Exec(
			`INSERT INTO order_items (order_id, product_id, name, quantity, price)
			 VALUES ($1, $2, $3, $4, $5)`,
			order.ID, order.Items[i].ProductID, order.Items[i].Name, order.Items[i].Quantity, order.Items[i].Price,
		)
		if err != nil {
			http.Error(w, "Failed to create order items", http.StatusInternalServerError)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		http.Error(w, "Failed to commit transaction", http.StatusInternalServerError)
		return
	}

	order.Status = "pending"
	order.PaymentStatus = "pending"

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
		err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.TotalAmount, &o.ShippingAddr, &o.PaymentMethod, &o.PaymentStatus, &o.CreatedAt, &o.UpdatedAt)
		if err != nil {
			continue
		}
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

	// Get order items
	rows, err := db.Query(
		"SELECT id, order_id, product_id, name, quantity, price FROM order_items WHERE order_id = $1",
		orderID,
	)
	if err == nil {
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

func updateOrderStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["id"]

	var update struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	validStatuses := map[string]bool{
		"pending":    true,
		"confirmed":  true,
		"processing": true,
		"shipped":    true,
		"delivered":  true,
		"cancelled":  true,
	}

	if !validStatuses[update.Status] {
		http.Error(w, "Invalid status", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(
		"UPDATE orders SET status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2",
		update.Status, orderID,
	)
	if err != nil {
		http.Error(w, "Failed to update order status", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Order status updated", "status": update.Status})
}

func updatePaymentStatus(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["id"]

	var update struct {
		PaymentStatus string `json:"payment_status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	validStatuses := map[string]bool{
		"pending":   true,
		"completed": true,
		"failed":    true,
		"refunded":  true,
	}

	if !validStatuses[update.PaymentStatus] {
		http.Error(w, "Invalid payment status", http.StatusBadRequest)
		return
	}

	_, err := db.Exec(
		"UPDATE orders SET payment_status = $1, updated_at = CURRENT_TIMESTAMP WHERE id = $2",
		update.PaymentStatus, orderID,
	)
	if err != nil {
		http.Error(w, "Failed to update payment status", http.StatusInternalServerError)
		return
	}

	// If payment is completed, update order status to confirmed
	if update.PaymentStatus == "completed" {
		db.Exec("UPDATE orders SET status = 'confirmed' WHERE id = $1", orderID)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Payment status updated", "payment_status": update.PaymentStatus})
}
