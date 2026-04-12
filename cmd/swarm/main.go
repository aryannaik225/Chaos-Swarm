package main

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

func generateToken() string {
	b := make([]byte, 8)
	rand.Read(b)
	return fmt.Sprintf("SWARM-%x", b)
}

func main() {
	fmt.Println("CHAOS SWARM - High-Concurrency HTTP Load Tester")
	fmt.Println("===============================================")

	targetDomain := "http://localhost:8080"
	targetAPI := targetDomain + "/api/checkout"
	authURL := targetDomain + "/swarm-auth.txt"

	token := generateToken()
	fmt.Printf("\n[SECURITY LOCK ACTIVE]\n")
	fmt.Printf("To authoize this attack, create a file named 'swarm-auth.txt' inside your 'cmd/target/static' folder.\n")
	fmt.Printf("Paste the following token inside it and save the file:\n\n")
	fmt.Printf("👉  %s  👈\n\n", token)

	fmt.Print("Press ENTER when the file is saved and you are ready to verify...")
	fmt.Scanln()

	resp, err := http.Get(authURL)
	if err != nil || resp.StatusCode != 200 {
		fmt.Println("\n ERROR: Could not find swarm-auth.txt on the target server. Attack aborted.")
		os.Exit(1)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(bodyBytes)) != token {
		fmt.Println("\n ERROR: Token mismatch. You do not have authorization to test this server. Attack aborted.")
		os.Exit(1)
	}

	fmt.Println("\nAuthorization Confirmed. Target is valid.")
	fmt.Println("INITIATING SWARM IN 3 SECONDS...")
	time.Sleep(3 * time.Second)

	totalRequests := 3000

	var successCount int32
	var failCount int32

	var wg sync.WaitGroup
	startTime := time.Now()

	fmt.Printf("Unleashing %d concurrent requests to %s...\n", totalRequests, targetAPI)

	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			// Simulate HTTP request
			client := http.Client{Timeout: 5 * time.Second}
			res, err := client.Get(targetAPI)
			if err != nil  {
				atomic.AddInt32(&failCount, 1)
				return
			}
			defer res.Body.Close()
			if res.StatusCode == 200 {
				atomic.AddInt32(&successCount, 1)
			} else {
				atomic.AddInt32(&failCount, 1)
			}
			}()
	}

	wg.Wait()
	duration := time.Since(startTime)

	// --- 3. RESULTS ---
	fmt.Println("\n==================================================")
	fmt.Println("TEST COMPLETE")
	fmt.Printf("Time Elapsed:  %v\n", duration)
	fmt.Printf("Successful:    %d\n", successCount)
	fmt.Printf("Failed:        %d\n", failCount)
	fmt.Println("==================================================")
}