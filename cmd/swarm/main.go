package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Step struct {
	Method  string `json:"method"`
	URL     string `json:"url"`
	Payload string `json:"payload"`
}

type Scenario struct {
	Name  string `json:"name"`
	Steps []Step `json:"steps"`
}

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

	targetURL := flag.String("url", "http://localhost:8080/api/checkout", "Fallback target URL")
	totalRequests := flag.Int("requests", 10000, "Total number of bot journeys to execute")
	targetRPS := flag.Int("rps", 1000, "Target Requests Per Second (RPS)")
	scenarioFile := flag.String("scenario", "", "Path to scenario.json")

	flag.Parse()

	fmt.Printf("%s🌪️  CHAOS SWARM - Stateful HTTP Load Tester%s\n", colorCyan, colorReset)
	fmt.Println("==================================================")

	var activeScenario Scenario
	if *scenarioFile != "" {
		fileBytes, err := os.ReadFile(*scenarioFile)
		if err != nil {
			fmt.Printf("%s❌ ERROR: Could not read scenario file: %v%s\n", colorRed, err, colorReset)
			os.Exit(1)
		}
		json.Unmarshal(fileBytes, &activeScenario)
		fmt.Printf("📂 Loaded Scenario: %s%s%s (%d steps)\n", colorYellow, activeScenario.Name, colorReset, len(activeScenario.Steps))
	} else {
		activeScenario = Scenario{
			Name: "Single Endpoint Attack",
			Steps: []Step{{Method: "GET", URL: *targetURL, Payload: ""}},
		}
	}

	parsedURL, err := url.Parse(activeScenario.Steps[0].URL)
	if err != nil {
		fmt.Printf("\n%s❌ ERROR: Invalid Target URL.%s\n", colorRed, colorReset)
		os.Exit(1)
	}
	targetDomain := parsedURL.Scheme + "://" + parsedURL.Host
	authURL := targetDomain + "/swarm-auth.txt"

	token := generateToken()
	fmt.Printf("\n%s[🔒 SECURITY LOCK ACTIVE]%s\n", colorYellow, colorReset)
	fmt.Printf("To authorize this attack, create 'swarm-auth.txt' in your target server's root.\n")
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

	fmt.Printf("\n%s✅ Authorization Confirmed.%s\n", colorGreen, colorReset)
	fmt.Printf("%s🚀 INITIATING SWARM TELEMETRY IN 3 SECONDS...%s\n\n", colorCyan, colorReset)
	time.Sleep(3 * time.Second)

	// --- THE SWARM ENGINE ---
	delayBetweenRequests := time.Second / time.Duration(*targetRPS)
	var successCount, failCount, completedCount int32
	var wg sync.WaitGroup
	var latencyHistogram [10000]int32
	
	// NEW: Telemetry Arrays
	var statusHistogram [600]int32 // Tracks HTTP codes from 0 to 599
	stepFailures := make([]int32, len(activeScenario.Steps))

	sharedTransport := &http.Transport{
		MaxIdleConns:        10000,
		MaxIdleConnsPerHost: 10000,
	}

	startTime := time.Now()

	go func() {
		for {
			s := atomic.LoadInt32(&successCount)
			f := atomic.LoadInt32(&failCount)
			c := atomic.LoadInt32(&completedCount)

			fmt.Printf("\r\033[K%s[LIVE STATS]%s 🟢 Success: %s%d%s | 🔴 Failed: %s%d%s | ⏱️  Progress: %d/%d",
				colorCyan, colorReset, colorGreen, s, colorReset, colorRed, f, colorReset, c, *totalRequests)

			if c == int32(*totalRequests) {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	for i := 0; i < *totalRequests; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			jar, _ := cookiejar.New(nil)
			client := &http.Client{
				Timeout:   10 * time.Second,
				Transport: sharedTransport,
				Jar:       jar,
			}

			reqStart := time.Now()
			journeySuccess := true

			for stepIdx, step := range activeScenario.Steps {
				var req *http.Request
				var reqErr error

				if step.Payload != "" {
					req, reqErr = http.NewRequest(step.Method, step.URL, bytes.NewBufferString(step.Payload))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req, reqErr = http.NewRequest(step.Method, step.URL, nil)
				}

				if reqErr != nil {
					atomic.AddInt32(&stepFailures[stepIdx], 1)
					journeySuccess = false
					break
				}

				res, err := client.Do(req)
				
				if err != nil {
					atomic.AddInt32(&stepFailures[stepIdx], 1)
					journeySuccess = false
					break
				}

				// NEW: Record HTTP Status Code
				code := res.StatusCode
				if code >= 0 && code < 600 {
					atomic.AddInt32(&statusHistogram[code], 1)
				}

				// If it's a 4xx or 5xx error, mark the journey as failed at THIS specific step
				if code >= 400 {
					atomic.AddInt32(&stepFailures[stepIdx], 1)
					journeySuccess = false
					io.Copy(io.Discard, res.Body)
					res.Body.Close()
					break
				}
				io.Copy(io.Discard, res.Body)
				res.Body.Close()
			}

			reqDur := time.Since(reqStart)

			if journeySuccess {
				atomic.AddInt32(&successCount, 1)
				ms := reqDur.Milliseconds()
				if ms >= 0 && ms < 10000 {
					atomic.AddInt32(&latencyHistogram[ms], 1)
				} else if ms >= 10000 {
					atomic.AddInt32(&latencyHistogram[9999], 1)
				}
			} else {
				atomic.AddInt32(&failCount, 1)
			}
			atomic.AddInt32(&completedCount, 1)
		}()

		time.Sleep(delayBetweenRequests)
	}

	wg.Wait()
	duration := time.Since(startTime)

	// Math calculation for Percentiles
	var p50, p95, p99 int
	totalSuccess := atomic.LoadInt32(&successCount)

	if totalSuccess > 0 {
		target50 := int32(float64(totalSuccess) * 0.50)
		target95 := int32(float64(totalSuccess) * 0.95)
		target99 := int32(float64(totalSuccess) * 0.99)

		var runningCount int32 = 0
		for ms, count := range latencyHistogram {
			if count == 0 { continue }
			runningCount += count
			if p50 == 0 && runningCount >= target50 { p50 = ms }
			if p95 == 0 && runningCount >= target95 { p95 = ms }
			if p99 == 0 && runningCount >= target99 { p99 = ms }
		}
	}

	// --- FINAL TELEMETRY OUTPUT ---
	fmt.Printf("\n\n%s==================================================%s\n", colorCyan, colorReset)
	fmt.Printf("📊 TEST COMPLETE\n")
	fmt.Printf("Time Elapsed:  %v\n", duration)
	fmt.Printf("Scenario:      %s\n\n", activeScenario.Name)

	fmt.Printf("%sSuccessful:%s    %d\n", colorGreen, colorReset, successCount)
	fmt.Printf("%sFailed:%s        %d\n\n", colorRed, colorReset, failCount)

	// Step Breakdown
	fmt.Printf("🔍 STEP-BY-STEP FAILURE BREAKDOWN\n")
	for i, step := range activeScenario.Steps {
		fails := atomic.LoadInt32(&stepFailures[i])
		if fails > 0 {
			fmt.Printf("  ❌ Step %d [%s %s]: %s%d failures%s\n", i+1, step.Method, step.URL, colorRed, fails, colorReset)
		} else {
			fmt.Printf("  ✅ Step %d [%s %s]: 0 failures\n", i+1, step.Method, step.URL)
		}
	}

	// Status Code Breakdown
	fmt.Printf("\n📡 HTTP STATUS CODES\n")
	for code, count := range statusHistogram {
		if count > 0 {
			if code >= 200 && code < 300 {
				fmt.Printf("  %s[%d]: %d%s\n", colorGreen, code, count, colorReset)
			} else {
				fmt.Printf("  %s[%d]: %d%s\n", colorYellow, code, count, colorReset)
			}
		}
	}

	fmt.Printf("\n⏱️  LATENCY PERCENTILES (Full Journey)\n")
	fmt.Printf("  P50 (Median):  %d ms\n", p50)
	fmt.Printf("  P95:           %d ms\n", p95)
	fmt.Printf("  P99:           %d ms\n", p99)
	fmt.Printf("%s==================================================%s\n", colorCyan, colorReset)
}