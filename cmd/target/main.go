package main

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

func main() {
	fs := http.FileServer(http.Dir("./cmd/target/static"))
	http.Handle("/", fs)

	// --- THE BOTTLENECK (Fake Database Connection Pool) ---
	// This channel only holds 50 "tokens". It is our database limit.
	dbPool := make(chan struct{}, 50)

	http.HandleFunc("/api/checkout", func(w http.ResponseWriter, r *http.Request) {
		select {
		case dbPool <- struct{}{}:
			defer func() { <-dbPool }() // Give the token back when we finish

			time.Sleep(3 * time.Second) // Simulate heavy database work
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status": "success", "message": "Ticket secured!"}`))

		default:
			// CRASH: The pool is completely full (50 active connections). 
			// We immediately reject the request. This is what causes the 502/503 errors!
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadGateway) // 502 Error
			w.Write([]byte(`{"error": "Connection Refused. Server exhausted pool limits."}`))
		}
	})

	port := ":8080"
	fmt.Printf("🎤 Taylor Swift Ticket Portal running on http://localhost%s\n", port)
	fmt.Println("⚠️  Warning: Strict DB Connection Pool (Max 50) is ACTIVE.")
	
	log.Fatal(http.ListenAndServe(port, nil))
}