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

	http.HandleFunc("/api/checkout", func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status": "success", "message": "Ticket secured!"}`))
	})

	http.HandleFunc("/swarm-auth.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`READY_FOR_SWARM`))
	})

	port := ":8080"
	fmt.Printf("Taylor Swift Ticket Portal running on http://localhost%s\n", port)
	fmt.Println("Warning: This server is intentionally fragile. Ready for load testing.")

	log.Fatal(http.ListenAndServe(port, nil))
}