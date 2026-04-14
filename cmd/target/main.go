package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// authMiddleware is a wrapper that checks if the request has a valid session cookie
// before allowing it to reach the actual API route.
func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("session_token")
		if err != nil || cookie.Value != "authenticated_bot" {
			// If there is no cookie, or the cookie is wrong, reject the request!
			http.Error(w, `{"error": "Unauthorized - Missing Session"}`, http.StatusUnauthorized)
			return
		}
		// Cookie is good, proceed to the route
		next(w, r)
	}
}

func main() {
	// Serve the Tailwind frontend
	fs := http.FileServer(http.Dir("./cmd/target/static"))
	http.Handle("/", fs)

	// --- 1. THE LOGIN ROUTE (Issues the Cookie) ---
	http.HandleFunc("/api/login", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		
		// Simulate password hashing delay
		time.Sleep(100 * time.Millisecond)

		// Set the stateful session cookie!
		http.SetCookie(w, &http.Cookie{
			Name:  "session_token",
			Value: "authenticated_bot",
			Path:  "/",
		})
		
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "success", "message": "Logged in successfully"}`))
	})

	// --- 2. THE CART ROUTE (Protected by Middleware) ---
	http.HandleFunc("/api/cart", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		// Simulate a fast database read
		time.Sleep(50 * time.Millisecond)
		
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status": "success", "items": [{"id": "VIP-13", "qty": 1}]}`))
	}))

	// --- 3. THE CHECKOUT ROUTE (Protected + The 50-Connection Trap) ---
	dbPool := make(chan struct{}, 50)

	http.HandleFunc("/api/checkout", authMiddleware(func(w http.ResponseWriter, r *http.Request) {
		select {
		case dbPool <- struct{}{}:
			// SUCCESS: We got a connection!
			defer func() { <-dbPool }() // Give token back

			time.Sleep(3 * time.Second) // The heavy DB lock trap
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "message": "Ticket secured!"}`))

		default:
			// CRASH: The pool is completely full.
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway) // 502 Error
			w.Write([]byte(`{"error": "Connection Refused. Server exhausted pool limits."}`))
		}
	}))

	port := ":8080"
	fmt.Printf("🎤 Taylor Swift Ticket Portal running on http://localhost%s\n", port)
	fmt.Println("⚠️  Warning: Strict DB Connection Pool (Max 50) is ACTIVE.")
	fmt.Println("🔐 Stateful Auth Routes (/api/login, /api/cart) are ACTIVE.")
	
	log.Fatal(http.ListenAndServe(port, nil))
}