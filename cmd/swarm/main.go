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
	colorReset := "\033[0m"
	colorRed := "\033[31m"
	colorGreen := "\033[32m"
	colorCyan := "\033[36m"
	colorYellow := "\033[33m"

	fmt.Printf("%s🌪️  CHAOS SWARM - High-Concurrency HTTP Load Tester%s\n", colorCyan, colorReset)
	fmt.Println("==================================================")

	targetDomain := "http://localhost:8080"
	targetAPI := targetDomain + "/api/checkout"
	authURL := targetDomain + "/swarm-auth.txt"

	token := generateToken()
	fmt.Printf("\n%s[🔒 SECURITY LOCK ACTIVE]%s\n", colorYellow, colorReset)
	fmt.Printf("To authorize this attack, create a file named 'swarm-auth.txt' inside your 'cmd/target/static' folder.\n")
	fmt.Printf("Paste the following token inside it and save the file:\n\n")
	fmt.Printf("👉  %s%s%s  👈\n\n", colorCyan, token, colorReset)
	
	fmt.Print("Press ENTER when the file is saved and you are ready to verify...")
	fmt.Scanln()

	resp, err := http.Get(authURL)
	if err != nil || resp.StatusCode != 200 {
		fmt.Printf("\n%s❌ ERROR: Could not find swarm-auth.txt on the target server. Attack aborted.%s\n", colorRed, colorReset)
		os.Exit(1)
	}
	defer resp.Body.Close()
	
	bodyBytes, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(bodyBytes)) != token {
		fmt.Printf("\n%s❌ ERROR: Token mismatch. You do not have authorization.%s\n", colorRed, colorReset)
		os.Exit(1)
	}

	fmt.Printf("\n%s✅ Authorization Confirmed. Target is valid.%s\n", colorGreen, colorReset)
	fmt.Printf("%s🚀 INITIATING SWARM IN 3 SECONDS...%s\n\n", colorCyan, colorReset)
	time.Sleep(3 * time.Second)

	// --- 2. THE SWARM ENGINE (Now with Rate Limiting!) ---
	totalRequests := 10000       // Pumped up to 10k!
	targetRPS := 1000            // Fire 1,000 requests per second
	delayBetweenRequests := time.Second / time.Duration(targetRPS)

	var successCount int32
	var failCount int32
	var completedCount int32
	var wg sync.WaitGroup

	startTime := time.Now()

	// UI Updater Goroutine (Runs at 20 FPS)
	go func() {
		for {
			s := atomic.LoadInt32(&successCount)
			f := atomic.LoadInt32(&failCount)
			c := atomic.LoadInt32(&completedCount)
			
			fmt.Printf("\r\033[K%s[LIVE STATS]%s 🟢 Success: %s%d%s | 🔴 Failed: %s%d%s | ⏱️  Progress: %d/%d", 
				colorCyan, colorReset, 
				colorGreen, s, colorReset, 
				colorRed, f, colorReset, 
				c, totalRequests)
			
			if c == int32(totalRequests) {
				break
			}
			time.Sleep(50 * time.Millisecond) 
		}
	}()

	// Fire the Worker Goroutines with Pacing
	for i := 0; i < totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			
			client := http.Client{Timeout: 5 * time.Second}
			res, err := client.Get(targetAPI)
			
			if err != nil {
				atomic.AddInt32(&failCount, 1)
			} else {
				if res.StatusCode == 200 {
					atomic.AddInt32(&successCount, 1)
				} else {
					atomic.AddInt32(&failCount, 1)
				}
				res.Body.Close()
			}
			atomic.AddInt32(&completedCount, 1)
		}()
		
		// The Rate Limiter: Pause briefly before launching the next bot
		time.Sleep(delayBetweenRequests)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// --- 3. FINAL RESULTS ---
	fmt.Printf("\n\n%s==================================================%s\n", colorCyan, colorReset)
	fmt.Printf("📊 TEST COMPLETE\n")
	fmt.Printf("Time Elapsed:  %v\n", duration)
	fmt.Printf("Target API:    %s\n", targetAPI)
	fmt.Printf("%sSuccessful:%s    %d\n", colorGreen, colorReset, successCount)
	fmt.Printf("%sFailed:%s        %d\n", colorRed, colorReset, failCount)
	fmt.Printf("%s==================================================%s\n", colorCyan, colorReset)
}