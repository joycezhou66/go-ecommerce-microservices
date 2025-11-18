package main

import (
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/joycezhou/go-ecommerce-microservices/shared/middleware"
)

type ServiceConfig struct {
	Name string
	URL  string
}

var services = map[string]string{
	"user":         getEnv("USER_SERVICE_URL", "http://user-service:8001"),
	"product":      getEnv("PRODUCT_SERVICE_URL", "http://product-service:8002"),
	"cart":         getEnv("CART_SERVICE_URL", "http://cart-service:8003"),
	"order":        getEnv("ORDER_SERVICE_URL", "http://order-service:8004"),
	"payment":      getEnv("PAYMENT_SERVICE_URL", "http://payment-service:8005"),
	"notification": getEnv("NOTIFICATION_SERVICE_URL", "http://notification-service:8006"),
}

func main() {
	r := mux.NewRouter()
	r.Use(middleware.CORS)
	r.Use(loggingMiddleware)
	r.Use(rateLimitMiddleware)

	// Health check
	r.HandleFunc("/health", healthCheck).Methods("GET")
	r.HandleFunc("/api/health", aggregateHealthCheck).Methods("GET")

	// User service routes
	r.PathPrefix("/api/users").HandlerFunc(proxyHandler("user"))
	r.HandleFunc("/api/register", proxyHandler("user")).Methods("POST")
	r.HandleFunc("/api/login", proxyHandler("user")).Methods("POST")

	// Product service routes
	r.PathPrefix("/api/products").HandlerFunc(proxyHandler("product"))
	r.PathPrefix("/api/categories").HandlerFunc(proxyHandler("product"))

	// Cart service routes
	r.PathPrefix("/api/cart").HandlerFunc(proxyHandler("cart"))

	// Order service routes
	r.PathPrefix("/api/orders").HandlerFunc(proxyHandler("order"))

	// Payment service routes
	r.PathPrefix("/api/payments").HandlerFunc(proxyHandler("payment"))

	// Notification service routes
	r.PathPrefix("/api/notifications").HandlerFunc(proxyHandler("notification"))

	// Serve static files for frontend
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./frontend/static"))))
	r.PathPrefix("/").Handler(http.FileServer(http.Dir("./frontend/templates")))

	log.Println("API Gateway running on :8080")
	log.Fatal(http.ListenAndServe(":8080", r))
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.Write([]byte(`{"status":"healthy","service":"gateway"}`))
}

func aggregateHealthCheck(w http.ResponseWriter, r *http.Request) {
	results := make(map[string]string)
	client := &http.Client{Timeout: 2 * time.Second}

	for name, serviceURL := range services {
		resp, err := client.Get(serviceURL + "/health")
		if err != nil {
			results[name] = "unhealthy"
			continue
		}
		resp.Body.Close()
		if resp.StatusCode == http.StatusOK {
			results[name] = "healthy"
		} else {
			results[name] = "unhealthy"
		}
	}

	w.Header().Set("Content-Type", "application/json")
	response := `{"gateway":"healthy","services":{`
	i := 0
	for name, status := range results {
		if i > 0 {
			response += ","
		}
		response += `"` + name + `":"` + status + `"`
		i++
	}
	response += "}}"
	w.Write([]byte(response))
}

func proxyHandler(serviceName string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		serviceURL, ok := services[serviceName]
		if !ok {
			http.Error(w, "Service not found", http.StatusNotFound)
			return
		}

		target, err := url.Parse(serviceURL)
		if err != nil {
			http.Error(w, "Invalid service URL", http.StatusInternalServerError)
			return
		}

		proxy := httputil.NewSingleHostReverseProxy(target)
		proxy.Director = func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host

			// Rewrite path
			path := r.URL.Path
			path = strings.TrimPrefix(path, "/api")

			// Map routes to service endpoints
			switch serviceName {
			case "user":
				if strings.HasPrefix(path, "/register") || strings.HasPrefix(path, "/login") {
					req.URL.Path = path
				} else {
					req.URL.Path = path
				}
			default:
				req.URL.Path = path
			}

			req.URL.RawQuery = r.URL.RawQuery

			// Copy headers
			for key, values := range r.Header {
				for _, value := range values {
					req.Header.Set(key, value)
				}
			}
		}

		proxy.ModifyResponse = func(resp *http.Response) error {
			resp.Header.Set("X-Gateway", "go-ecommerce")
			return nil
		}

		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			log.Printf("Proxy error for %s: %v", serviceName, err)
			http.Error(w, "Service unavailable", http.StatusServiceUnavailable)
		}

		proxy.ServeHTTP(w, r)
	}
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Create a response writer wrapper to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(wrapped, r)

		log.Printf(
			"%s %s %d %s",
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			time.Since(start),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

// Simple rate limiter
var requestCounts = make(map[string]int)
var lastReset = time.Now()

func rateLimitMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Reset counts every minute
		if time.Since(lastReset) > time.Minute {
			requestCounts = make(map[string]int)
			lastReset = time.Now()
		}

		ip := r.RemoteAddr
		requestCounts[ip]++

		if requestCounts[ip] > 1000 { // 1000 requests per minute
			http.Error(w, "Rate limit exceeded", http.StatusTooManyRequests)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func copyBody(dst io.Writer, src io.Reader) {
	io.Copy(dst, src)
}
