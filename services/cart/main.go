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

var db *sql.DB

func main() {
	var err error
	db, err = database.NewConnection("cart_db")
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}
	defer db.Close()

	initDB()

	r := mux.NewRouter()
	r.Use(middleware.CORS)

	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/cart/{user_id}", getCart).Methods("GET")
	r.HandleFunc("/cart/{user_id}/items", addToCart).Methods("POST")
	r.HandleFunc("/cart/{user_id}/items/{item_id}", updateCartItem).Methods("PUT")
	r.HandleFunc("/cart/{user_id}/items/{item_id}", removeFromCart).Methods("DELETE")
	r.HandleFunc("/cart/{user_id}", clearCart).Methods("DELETE")

	log.Println("Cart service running on :8003")
	log.Fatal(http.ListenAndServe(":8003", r))
}

func initDB() {
	query := `
	CREATE TABLE IF NOT EXISTS cart_items (
		id SERIAL PRIMARY KEY,
		user_id INT NOT NULL,
		product_id INT NOT NULL,
		quantity INT NOT NULL DEFAULT 1,
		price DECIMAL(10,2) NOT NULL,
		name VARCHAR(255) NOT NULL,
		image_url TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(user_id, product_id)
	)`
	_, err := db.Exec(query)
	if err != nil {
		log.Fatal("Failed to create cart_items table:", err)
	}
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	json.NewEncoder(w).Encode(map[string]string{"status": "healthy"})
}

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

	// Try to update existing item, if not exists then insert
	result, err := db.Exec(
		`INSERT INTO cart_items (user_id, product_id, quantity, price, name, image_url)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 ON CONFLICT (user_id, product_id) DO UPDATE SET quantity = cart_items.quantity + $3`,
		userID, item.ProductID, item.Quantity, item.Price, item.Name, item.ImageURL,
	)

	if err != nil {
		http.Error(w, "Failed to add item to cart", http.StatusInternalServerError)
		return
	}

	rowsAffected, _ := result.RowsAffected()
	w.Header().Set("Content-Type", "application/json")
	if rowsAffected > 0 {
		json.NewEncoder(w).Encode(map[string]string{"message": "Item added to cart"})
	} else {
		http.Error(w, "Failed to add item", http.StatusInternalServerError)
	}
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
		// Remove item if quantity is 0 or less
		_, err := db.Exec("DELETE FROM cart_items WHERE id = $1 AND user_id = $2", itemID, userID)
		if err != nil {
			http.Error(w, "Failed to remove item", http.StatusInternalServerError)
			return
		}
	} else {
		_, err := db.Exec(
			"UPDATE cart_items SET quantity = $1 WHERE id = $2 AND user_id = $3",
			update.Quantity, itemID, userID,
		)
		if err != nil {
			http.Error(w, "Failed to update item", http.StatusInternalServerError)
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"message": "Cart updated"})
}

func removeFromCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]
	itemID := vars["item_id"]

	_, err := db.Exec("DELETE FROM cart_items WHERE id = $1 AND user_id = $2", itemID, userID)
	if err != nil {
		http.Error(w, "Failed to remove item", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func clearCart(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	userID := vars["user_id"]

	_, err := db.Exec("DELETE FROM cart_items WHERE user_id = $1", userID)
	if err != nil {
		http.Error(w, "Failed to clear cart", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func GetCartItemsByUserID(userID string) ([]CartItem, error) {
	rows, err := db.Query(
		`SELECT id, user_id, product_id, quantity, price, name, image_url, created_at
		 FROM cart_items WHERE user_id = $1`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []CartItem
	for rows.Next() {
		var item CartItem
		rows.Scan(&item.ID, &item.UserID, &item.ProductID, &item.Quantity, &item.Price, &item.Name, &item.ImageURL, &item.CreatedAt)
		items = append(items, item)
	}
	return items, nil
}

func ClearCartByUserID(userID string) error {
	_, err := db.Exec("DELETE FROM cart_items WHERE user_id = $1", userID)
	return err
}

func GetTotalPrice(userID string) (float64, error) {
	var total float64
	err := db.QueryRow(
		"SELECT COALESCE(SUM(price * quantity), 0) FROM cart_items WHERE user_id = $1",
		userID,
	).Scan(&total)
	return total, err
}

func GetUserIDFromContext(r *http.Request) string {
	return strconv.Itoa(int(r.Context().Value("user_id").(uint)))
}
