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

type Notification struct {
	ID        uint      `json:"id"`
	UserID    uint      `json:"user_id"`
	Type      string    `json:"type"`
	Channel   string    `json:"channel"`
	Subject   string    `json:"subject"`
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	Metadata  string    `json:"metadata,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
}

type NotificationRequest struct {
	UserID   uint   `json:"user_id"`
	Type     string `json:"type"`
	Channel  string `json:"channel"`
	Subject  string `json:"subject"`
	Message  string `json:"message"`
	Metadata string `json:"metadata,omitempty"`
}

var db *sql.DB

func main() {
	var err error
	db, err = database.NewConnection("notifications_db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	initDB()

	r := mux.NewRouter()
	r.Use(middleware.CORS)

	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/notifications", sendNotification).Methods("POST")
	r.HandleFunc("/notifications/user/{user_id}", getNotificationsByUser).Methods("GET")
	r.HandleFunc("/notifications/{id}", getNotification).Methods("GET")
	r.HandleFunc("/notifications/bulk", sendBulkNotifications).Methods("POST")

	// Template endpoints
	r.HandleFunc("/notifications/order-confirmation", sendOrderConfirmation).Methods("POST")
	r.HandleFunc("/notifications/shipping-update", sendShippingUpdate).Methods("POST")
	r.HandleFunc("/notifications/payment-receipt", sendPaymentReceipt).Methods("POST")

	log.Println("Notification service running on :8006")
	log.Fatal(http.ListenAndServe(":8006", r))
}

func initDB() {
	query := `
	CREATE TABLE IF NOT EXISTS notifications (
		id SERIAL PRIMARY KEY,
		user_id INT NOT NULL,
		type VARCHAR(50) NOT NULL,
		channel VARCHAR(20) NOT NULL,
		subject VARCHAR(255),
		message TEXT NOT NULL,
		status VARCHAR(20) DEFAULT 'pending',
		metadata JSONB,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		sent_at TIMESTAMP
	)`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Failed to create notifications table:", err)
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

func sendNotification(w http.ResponseWriter, r *http.Request) {
	var req NotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	notification := Notification{
		UserID:  req.UserID,
		Type:    req.Type,
		Channel: req.Channel,
		Subject: req.Subject,
		Message: req.Message,
		Status:  "pending",
	}

	// Simulate sending notification
	notification.Status = "sent"
	sentAt := time.Now()
	notification.SentAt = &sentAt

	err := db.QueryRow(
		`INSERT INTO notifications (user_id, type, channel, subject, message, status, metadata, sent_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id, created_at`,
		notification.UserID, notification.Type, notification.Channel, notification.Subject, notification.Message, notification.Status, req.Metadata, notification.SentAt,
	).Scan(&notification.ID, &notification.CreatedAt)

	if err != nil {
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return
	}

	log.Printf("Notification sent: [%s] %s to user %d via %s", notification.Type, notification.Subject, notification.UserID, notification.Channel)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(notification)
}

func getNotificationsByUser(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	rows, err := db.Query(
		`SELECT id, user_id, type, channel, subject, message, status, metadata, created_at, sent_at
		 FROM notifications WHERE user_id = $1 ORDER BY created_at DESC LIMIT 100`,
		userID,
	)
	if err != nil {
		http.Error(w, "Failed to fetch notifications", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	notifications := []Notification{}
	for rows.Next() {
		var n Notification
		var metadata sql.NullString
		var sentAt sql.NullTime
		rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Channel, &n.Subject, &n.Message, &n.Status, &metadata, &n.CreatedAt, &sentAt)
		if metadata.Valid {
			n.Metadata = metadata.String
		}
		if sentAt.Valid {
			n.SentAt = &sentAt.Time
		}
		notifications = append(notifications, n)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(notifications)
}

func getNotification(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	notificationID := vars["id"]

	var n Notification
	var metadata sql.NullString
	var sentAt sql.NullTime
	err := db.QueryRow(
		`SELECT id, user_id, type, channel, subject, message, status, metadata, created_at, sent_at
		 FROM notifications WHERE id = $1`,
		notificationID,
	).Scan(&n.ID, &n.UserID, &n.Type, &n.Channel, &n.Subject, &n.Message, &n.Status, &metadata, &n.CreatedAt, &sentAt)

	if err != nil {
		http.Error(w, "Notification not found", http.StatusNotFound)
		return
	}

	if metadata.Valid {
		n.Metadata = metadata.String
	}
	if sentAt.Valid {
		n.SentAt = &sentAt.Time
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(n)
}

func sendBulkNotifications(w http.ResponseWriter, r *http.Request) {
	var requests []NotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&requests); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	results := make([]map[string]interface{}, len(requests))
	for i, req := range requests {
		sentAt := time.Now()
		var id uint
		err := db.QueryRow(
			`INSERT INTO notifications (user_id, type, channel, subject, message, status, metadata, sent_at)
			 VALUES ($1, $2, $3, $4, $5, 'sent', $6, $7) RETURNING id`,
			req.UserID, req.Type, req.Channel, req.Subject, req.Message, req.Metadata, sentAt,
		).Scan(&id)

		if err != nil {
			results[i] = map[string]interface{}{"success": false, "error": err.Error()}
		} else {
			results[i] = map[string]interface{}{"success": true, "id": id}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"results": results})
}

func sendOrderConfirmation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID   uint    `json:"user_id"`
		OrderID  uint    `json:"order_id"`
		Email    string  `json:"email"`
		Total    float64 `json:"total"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	notification := NotificationRequest{
		UserID:  req.UserID,
		Type:    "order_confirmation",
		Channel: "email",
		Subject: "Order Confirmation",
		Message: formatOrderConfirmation(req.OrderID, req.Total),
	}

	sentAt := time.Now()
	var id uint
	err := db.QueryRow(
		`INSERT INTO notifications (user_id, type, channel, subject, message, status, sent_at)
		 VALUES ($1, $2, $3, $4, $5, 'sent', $6) RETURNING id`,
		notification.UserID, notification.Type, notification.Channel, notification.Subject, notification.Message, sentAt,
	).Scan(&id)

	if err != nil {
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return
	}

	log.Printf("Order confirmation sent for order #%d to user %d", req.OrderID, req.UserID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "status": "sent"})
}

func sendShippingUpdate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID        uint   `json:"user_id"`
		OrderID       uint   `json:"order_id"`
		Status        string `json:"status"`
		TrackingNumber string `json:"tracking_number"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	notification := NotificationRequest{
		UserID:  req.UserID,
		Type:    "shipping_update",
		Channel: "email",
		Subject: "Shipping Update",
		Message: formatShippingUpdate(req.OrderID, req.Status, req.TrackingNumber),
	}

	sentAt := time.Now()
	var id uint
	err := db.QueryRow(
		`INSERT INTO notifications (user_id, type, channel, subject, message, status, sent_at)
		 VALUES ($1, $2, $3, $4, $5, 'sent', $6) RETURNING id`,
		notification.UserID, notification.Type, notification.Channel, notification.Subject, notification.Message, sentAt,
	).Scan(&id)

	if err != nil {
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "status": "sent"})
}

func sendPaymentReceipt(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID        uint    `json:"user_id"`
		OrderID       uint    `json:"order_id"`
		Amount        float64 `json:"amount"`
		TransactionID string  `json:"transaction_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	notification := NotificationRequest{
		UserID:  req.UserID,
		Type:    "payment_receipt",
		Channel: "email",
		Subject: "Payment Receipt",
		Message: formatPaymentReceipt(req.OrderID, req.Amount, req.TransactionID),
	}

	sentAt := time.Now()
	var id uint
	err := db.QueryRow(
		`INSERT INTO notifications (user_id, type, channel, subject, message, status, sent_at)
		 VALUES ($1, $2, $3, $4, $5, 'sent', $6) RETURNING id`,
		notification.UserID, notification.Type, notification.Channel, notification.Subject, notification.Message, sentAt,
	).Scan(&id)

	if err != nil {
		http.Error(w, "Failed to send notification", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{"id": id, "status": "sent"})
}

func formatOrderConfirmation(orderID uint, total float64) string {
	return "Thank you for your order #" + string(rune(orderID)) + "! Your order total is $" + formatFloat(total) + ". We'll notify you when it ships."
}

func formatShippingUpdate(orderID uint, status, trackingNumber string) string {
	msg := "Your order #" + string(rune(orderID)) + " has been " + status + "."
	if trackingNumber != "" {
		msg += " Tracking number: " + trackingNumber
	}
	return msg
}

func formatPaymentReceipt(orderID uint, amount float64, transactionID string) string {
	return "Payment of $" + formatFloat(amount) + " received for order #" + string(rune(orderID)) + ". Transaction ID: " + transactionID
}

func formatFloat(f float64) string {
	return string(rune(int(f))) + "." + string(rune(int((f-float64(int(f)))*100)))
}
