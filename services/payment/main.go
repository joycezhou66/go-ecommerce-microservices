package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gorilla/mux"
	"github.com/joycezhou/go-ecommerce-microservices/shared/database"
	"github.com/joycezhou/go-ecommerce-microservices/shared/middleware"
)

type Payment struct {
	ID              uint      `json:"id"`
	OrderID         uint      `json:"order_id"`
	UserID          uint      `json:"user_id"`
	Amount          float64   `json:"amount"`
	Currency        string    `json:"currency"`
	Method          string    `json:"method"`
	Status          string    `json:"status"`
	TransactionID   string    `json:"transaction_id"`
	PaymentGateway  string    `json:"payment_gateway"`
	CardLast4       string    `json:"card_last4,omitempty"`
	ErrorMessage    string    `json:"error_message,omitempty"`
	CreatedAt       time.Time `json:"created_at"`
}

type PaymentRequest struct {
	OrderID  uint    `json:"order_id"`
	UserID   uint    `json:"user_id"`
	Amount   float64 `json:"amount"`
	Currency string  `json:"currency"`
	Method   string  `json:"method"`
	CardInfo *struct {
		Number   string `json:"number"`
		ExpMonth string `json:"exp_month"`
		ExpYear  string `json:"exp_year"`
		CVC      string `json:"cvc"`
	} `json:"card_info,omitempty"`
}

var db *sql.DB

func main() {
	var err error
	db, err = database.NewConnection("payments_db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	initDB()

	r := mux.NewRouter()
	r.Use(middleware.CORS)

	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/payments", processPayment).Methods("POST")
	r.HandleFunc("/payments/{id}", getPayment).Methods("GET")
	r.HandleFunc("/payments/order/{order_id}", getPaymentByOrder).Methods("GET")
	r.HandleFunc("/payments/{id}/refund", refundPayment).Methods("POST")
	r.HandleFunc("/payments/user/{user_id}", getPaymentsByUser).Methods("GET")

	log.Println("Payment service running on :8005")
	log.Fatal(http.ListenAndServe(":8005", r))
}

func initDB() {
	query := `
	CREATE TABLE IF NOT EXISTS payments (
		id SERIAL PRIMARY KEY,
		order_id INT NOT NULL,
		user_id INT NOT NULL,
		amount DECIMAL(10,2) NOT NULL,
		currency VARCHAR(3) DEFAULT 'USD',
		method VARCHAR(50) NOT NULL,
		status VARCHAR(50) DEFAULT 'pending',
		transaction_id VARCHAR(100) UNIQUE,
		payment_gateway VARCHAR(50),
		card_last4 VARCHAR(4),
		error_message TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Failed to create payments table:", err)
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func processPayment(w http.ResponseWriter, r *http.Request) {
	var req PaymentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Currency == "" {
		req.Currency = "USD"
	}

	// Generate transaction ID
	transactionID := generateTransactionID()

	// Simulate payment processing
	payment := Payment{
		OrderID:        req.OrderID,
		UserID:         req.UserID,
		Amount:         req.Amount,
		Currency:       req.Currency,
		Method:         req.Method,
		TransactionID:  transactionID,
		PaymentGateway: "stripe_simulator",
	}

	// Get last 4 digits of card if provided
	if req.CardInfo != nil && len(req.CardInfo.Number) >= 4 {
		payment.CardLast4 = req.CardInfo.Number[len(req.CardInfo.Number)-4:]
	}

	// Simulate payment gateway response (90% success rate)
	if rand.Float32() < 0.9 {
		payment.Status = "completed"
	} else {
		payment.Status = "failed"
		payment.ErrorMessage = "Payment declined by issuer"
	}

	err := db.QueryRow(
		`INSERT INTO payments (order_id, user_id, amount, currency, method, status, transaction_id, payment_gateway, card_last4, error_message)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) RETURNING id, created_at`,
		payment.OrderID, payment.UserID, payment.Amount, payment.Currency, payment.Method, payment.Status, payment.TransactionID, payment.PaymentGateway, payment.CardLast4, payment.ErrorMessage,
	).Scan(&payment.ID, &payment.CreatedAt)

	if err != nil {
		http.Error(w, "Failed to process payment", http.StatusInternalServerError)
		return
	}

	// Update order payment status
	if payment.Status == "completed" {
		updateOrderPaymentStatus(payment.OrderID, "completed")
	} else {
		updateOrderPaymentStatus(payment.OrderID, "failed")
	}

	w.Header().Set("Content-Type", "application/json")
	if payment.Status == "completed" {
		w.WriteHeader(http.StatusCreated)
	} else {
		w.WriteHeader(http.StatusPaymentRequired)
	}
	json.NewEncoder(w).Encode(payment)
}

func getPayment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paymentID := vars["id"]

	var payment Payment
	err := db.QueryRow(
		`SELECT id, order_id, user_id, amount, currency, method, status, transaction_id, payment_gateway, card_last4, error_message, created_at
		 FROM payments WHERE id = $1`,
		paymentID,
	).Scan(&payment.ID, &payment.OrderID, &payment.UserID, &payment.Amount, &payment.Currency, &payment.Method, &payment.Status, &payment.TransactionID, &payment.PaymentGateway, &payment.CardLast4, &payment.ErrorMessage, &payment.CreatedAt)

	if err != nil {
		http.Error(w, "Payment not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payment)
}

func getPaymentByOrder(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	orderID := vars["order_id"]

	var payment Payment
	err := db.QueryRow(
		`SELECT id, order_id, user_id, amount, currency, method, status, transaction_id, payment_gateway, card_last4, error_message, created_at
		 FROM payments WHERE order_id = $1 ORDER BY created_at DESC LIMIT 1`,
		orderID,
	).Scan(&payment.ID, &payment.OrderID, &payment.UserID, &payment.Amount, &payment.Currency, &payment.Method, &payment.Status, &payment.TransactionID, &payment.PaymentGateway, &payment.CardLast4, &payment.ErrorMessage, &payment.CreatedAt)

	if err != nil {
		http.Error(w, "Payment not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payment)
}

func getPaymentsByUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	rows, err := db.Query(
		`SELECT id, order_id, user_id, amount, currency, method, status, transaction_id, payment_gateway, card_last4, error_message, created_at
		 FROM payments WHERE user_id = $1 ORDER BY created_at DESC`,
		userID,
	)
	if err != nil {
		http.Error(w, "Failed to fetch payments", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	payments := []Payment{}
	for rows.Next() {
		var p Payment
		rows.Scan(&p.ID, &p.OrderID, &p.UserID, &p.Amount, &p.Currency, &p.Method, &p.Status, &p.TransactionID, &p.PaymentGateway, &p.CardLast4, &p.ErrorMessage, &p.CreatedAt)
		payments = append(payments, p)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(payments)
}

func refundPayment(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	paymentID := vars["id"]

	var payment Payment
	err := db.QueryRow(
		"SELECT id, order_id, status FROM payments WHERE id = $1",
		paymentID,
	).Scan(&payment.ID, &payment.OrderID, &payment.Status)

	if err != nil {
		http.Error(w, "Payment not found", http.StatusNotFound)
		return
	}

	if payment.Status != "completed" {
		http.Error(w, "Only completed payments can be refunded", http.StatusBadRequest)
		return
	}

	_, err = db.Exec("UPDATE payments SET status = 'refunded' WHERE id = $1", paymentID)
	if err != nil {
		http.Error(w, "Failed to refund payment", http.StatusInternalServerError)
		return
	}

	updateOrderPaymentStatus(payment.OrderID, "refunded")

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Payment refunded successfully"})
}

func generateTransactionID() string {
	return fmt.Sprintf("txn_%d_%d", time.Now().UnixNano(), rand.Int63n(10000))
}

func updateOrderPaymentStatus(orderID uint, status string) {
	orderServiceURL := os.Getenv("ORDER_SERVICE_URL")
	if orderServiceURL == "" {
		orderServiceURL = "http://order-service:8004"
	}

	payload := map[string]string{"payment_status": status}
	jsonPayload, _ := json.Marshal(payload)

	req, _ := http.NewRequest("PATCH", fmt.Sprintf("%s/orders/%d/payment", orderServiceURL, orderID), bytes.NewBuffer(jsonPayload))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Second}
	client.Do(req)
}
