package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// enforce session cookie
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value != "authenticated_bot" {
			http.Error(w, `{"error": "Unauthorized"}`, http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func main() {
	// serve static assets
	fs := http.FileServer(http.Dir("./cmd/target/static"))
	http.Handle("/", fs)

	// mock login + set cookie
	http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		// fake hash delay
		time.Sleep(100 * time.Millisecond)

		http.SetCookie(w, &http.Cookie{
			Name:  "session_token",
			Value: "authenticated_bot",
			Path:  "/",
		})
		
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "success", "message": "Logged in"}`))
	})

	// protected cart route
	http.HandleFunc("/api/cart", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// fake db read
		time.Sleep(50 * time.Millisecond)
		
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "success", "items": [{"id": "VIP-13", "qty": 1}]}`))
	}))

	// checkout bottleneck sim (max 50 concurrent)
	dbPool := make(chan struct{}, 50)

	http.HandleFunc("/api/checkout", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		select {
		case dbPool <- struct{}{}:
			// got lock, release on exit
			defer func() { <-dbPool }() 

			// slow db query trap
			time.Sleep(3 * time.Second) 
			
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "message": "Ticket secured"}`))

		default:
			// pool exhausted
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway) 
			w.Write([]byte(`{"error": "Connection Refused"}`))
		}
	}))

	port := ":8080"
	fmt.Printf("[SERVER] Target API running on port %s\n", port)
	fmt.Println("[CONFIG] DB Connection Pool Limit: 50")
	fmt.Println("[CONFIG] Auth Routes Active")
	
	log.Fatal(http.ListenAndServe(port, nil))
}