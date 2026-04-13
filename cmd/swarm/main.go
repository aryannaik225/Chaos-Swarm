package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

	targetURL := flag.String("url", "http://localhost:8080/api/checkout", "Target API URL to attack")
	totalRequests := flag.Int("requests", 10000, "Total number of requests to send")
	targetRPS := flag.Int("rps", 1000, "Target Requests Per Second (RPS) to maintain")

	flag.Parse()

	fmt.Printf("%s🌪️  CHAOS SWARM - High-Concurrency HTTP Load Tester%s\n", colorCyan, colorReset)
	fmt.Println("==================================================")

	targetAPI := *targetURL
	parsedURL, err := url.Parse(targetAPI)
	if err != nil {
		fmt.Printf("\n%s❌ ERROR: Invalid Target URL format.%s\n", colorRed, colorReset)
		os.Exit(1)
	}
	targetDomain := parsedURL.Scheme + "://" + parsedURL.Host
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

	delayBetweenRequests := time.Second / time.Duration(*targetRPS)

	var successCount int32
	var failCount int32
	var completedCount int32
	var wg sync.WaitGroup

	var histogram [10000]int32

	startTime := time.Now()

	go func() {
		for {
			s := atomic.LoadInt32(&successCount)
			f := atomic.LoadInt32(&failCount)
			c := atomic.LoadInt32(&completedCount)

			fmt.Printf("\r\033[K%s[LIVE STATS]%s 🟢 Success: %s%d%s | 🔴 Failed: %s%d%s | ⏱️  Progress: %d/%d",
				colorCyan, colorReset,
				colorGreen, s, colorReset,
				colorRed, f, colorReset,
				c, *totalRequests)

			if c == int32(*totalRequests) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	sharedClient := http.Client{Timeout: 5 * time.Second}

	for i := 0; i < *totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			reqStart := time.Now()
			res, err := sharedClient.Get(targetAPI)
			reqDur := time.Since(reqStart)

			if err != nil {
				atomic.AddInt32(&failCount, 1)
			} else {
				if res.StatusCode == 200 {
					atomic.AddInt32(&successCount, 1)

					ms := reqDur.Milliseconds()
					if ms >= 0 && ms < 10000 {
						atomic.AddInt32(&histogram[ms], 1)
					} else if ms >= 10000 {
						atomic.AddInt32(&histogram[9999], 1)
					}
				} else {
					atomic.AddInt32(&failCount, 1)
				}
				res.Body.Close()
			}
			atomic.AddInt32(&completedCount, 1)
		}()

		time.Sleep(delayBetweenRequests)
	}

	wg.Wait()
	duration := time.Since(startTime)

	var p50, p95, p99 int
	totalSuccess := atomic.LoadInt32(&successCount)

	// Calculating percentiles from the histogram
	if totalSuccess > 0 {
		target50 := int32(float64(totalSuccess) * 0.50)
		target95 := int32(float64(totalSuccess) * 0.95)
		target99 := int32(float64(totalSuccess) * 0.99)

		var runningCount int32 = 0
		for ms, count := range histogram {
			if count == 0 {
				continue
			}
			runningCount += count
			if p50 == 0 && runningCount >= target50 {
				p50 = ms
			}
			if p95 == 0 && runningCount >= target95 {
				p95 = ms
			}
			if p99 == 0 && runningCount >= target99 {
				p99 = ms
			}
		}
	}

	fmt.Printf("\n\n%s==================================================%s\n", colorCyan, colorReset)
	fmt.Printf("📊 TEST COMPLETE\n")
	fmt.Printf("Time Elapsed:  %v\n", duration)
	fmt.Printf("Target API:    %s\n\n", targetAPI)

	fmt.Printf("%sSuccessful:%s    %d\n", colorGreen, colorReset, successCount)
	fmt.Printf("%sFailed:%s        %d\n\n", colorRed, colorReset, failCount)

	fmt.Printf("⏱️  LATENCY PERCENTILES (Successful Requests)\n")
	fmt.Printf("P50 (Median):  %d ms\n", p50)
	fmt.Printf("P95:           %d ms\n", p95)
	fmt.Printf("P99:           %d ms\n", p99)
	fmt.Printf("%s==================================================%s\n", colorCyan, colorReset)
}