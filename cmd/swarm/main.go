package main

import (
	"bytes"
	"crypto/rand"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/rpc"
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

type SwarmArgs struct {
	TargetURL    string
	TotalReqs    int
	TargetRPS    int
	ScenarioData Scenario
}

// payload worker sends back to master
type SwarmReply struct {
	SuccessCount     int32
	FailCount        int32
	LatencyHistogram [10000]int32
	StatusHistogram  [600]int32
	StepFailures     []int32
	Duration         time.Duration
}

type WorkerNode struct{}

func (w *WorkerNode) ExecuteSwarm(args SwarmArgs, reply *SwarmReply) error {
	fmt.Printf("\n[RECEIVED COMMAND] Executing %d requests at %d RPS...\n", args.TotalReqs, args.TargetRPS)

	delayBetweenRequests := time.Second / time.Duration(args.TargetRPS)
	
	var successCount, failCount, completedCount int32 
	var wg sync.WaitGroup
	
	// fixed arrays so gc doesn't hog the cpu
	var latencyHistogram [10000]int32
	var statusHistogram [600]int32
	stepFailures := make([]int32, len(args.ScenarioData.Steps))

	// reuse tcp connections to prevent port exhaustion
	sharedTransport := &http.Transport{
		MaxIdleConns:        10000,
		MaxIdleConnsPerHost: 10000,
	}

	startTime := time.Now()

	go func() {
		colorReset := "\033[0m"
		colorRed := "\033[31m"
		colorGreen := "\033[32m"
		colorCyan := "\033[36m"

		for {
			s := atomic.LoadInt32(&successCount)
			f := atomic.LoadInt32(&failCount)
			c := atomic.LoadInt32(&completedCount)

			fmt.Printf("\r\033[K%s[LIVE STATS]%s Success: %s%d%s | Failed: %s%d%s | Progress: %d/%d",
				colorCyan, colorReset, colorGreen, s, colorReset, colorRed, f, colorReset, c, args.TotalReqs)

			// stop loop if done
			if c == int32(args.TotalReqs) {
				fmt.Println() 
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
	}()

	for i := 0; i < args.TotalReqs; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			jar, _ := cookiejar.New(nil)
			client := &http.Client{Timeout: 10 * time.Second, Transport: sharedTransport, Jar: jar}

			reqStart := time.Now()
			journeySuccess := true

			for stepIdx, step := range args.ScenarioData.Steps {
				var req *http.Request
				var reqErr error

				if step.Payload != "" {
					req, reqErr = http.NewRequest(step.Method, step.URL, bytes.NewBufferString(step.Payload))
					req.Header.Set("Content-Type", "application/json")
				} else {
					req, reqErr = http.NewRequest(step.Method, step.URL, nil)
				}

				// bail out early if step crashes
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

				code := res.StatusCode
				if code >= 0 && code < 600 {
					atomic.AddInt32(&statusHistogram[code], 1)
				}

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
	
	fmt.Println("[Attack complete] Packaging telemetry data...")

	reply.Duration = time.Since(startTime)
	reply.SuccessCount = successCount
	reply.FailCount = failCount
	reply.LatencyHistogram = latencyHistogram
	reply.StatusHistogram = statusHistogram
	reply.StepFailures = stepFailures

	return nil
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

	runMode := flag.String("mode", "standalone", "Mode: 'standalone', 'master', or 'worker'")
	workerPort := flag.String("port", ":9000", "Port for worker to listen on")
	workerNodes := flag.String("workers", "localhost:9000", "Comma-separated worker addresses")
	
	targetURL := flag.String("url", "http://localhost:8080/api/checkout", "Fallback target URL")
	totalRequests := flag.Int("requests", 10000, "Total number of bot journeys to execute")
	targetRPS := flag.Int("rps", 1000, "Target Requests Per Second (RPS)")
	scenarioFile := flag.String("scenario", "", "Path to scenario.json")

	flag.Parse()

	fmt.Printf("%s[CHAOS SWARM] - Enterprise Distributed Load Tester%s\n", colorCyan, colorReset)
	fmt.Println("==================================================")

	if *runMode == "worker" {
		worker := new(WorkerNode)
		rpc.Register(worker)
		listener, err := net.Listen("tcp", *workerPort)
		if err != nil {
			fmt.Printf("%s[ERROR] Failed to start worker on %s%s\n", colorRed, *workerPort, colorReset)
			os.Exit(1)
		}
		fmt.Printf("%s[WORKER NODE ONLINE]%s - Listening on %s...\n", colorGreen, colorReset, *workerPort)
		
		for {
			conn, err := listener.Accept()
			if err != nil { continue }
			go rpc.ServeConn(conn)
		}
	}

	var activeScenario Scenario
	if *scenarioFile != "" {
		fileBytes, err := os.ReadFile(*scenarioFile)
		if err != nil { os.Exit(1) }
		json.Unmarshal(fileBytes, &activeScenario)
	} else {
		activeScenario = Scenario{
			Name: "Single Endpoint Attack",
			Steps: []Step{{Method: "GET", URL: *targetURL, Payload: ""}},
		}
	}

	parsedURL, _ := url.Parse(activeScenario.Steps[0].URL)
	targetDomain := parsedURL.Scheme + "://" + parsedURL.Host
	authURL := targetDomain + "/swarm-auth.txt"
	token := generateToken()

	fmt.Printf("\n%s[SECURITY LOCK ACTIVE]%s\n", colorYellow, colorReset)
	fmt.Printf(">>  %s%s%s  <<\n\n", colorCyan, token, colorReset)
	fmt.Print("Press ENTER when 'swarm-auth.txt' is saved on target...")
	fmt.Scanln()

	resp, err := http.Get(authURL)
	if err != nil || resp.StatusCode != 200 { os.Exit(1) }
	bodyBytes, _ := io.ReadAll(resp.Body)
	if strings.TrimSpace(string(bodyBytes)) != token { os.Exit(1) }
	resp.Body.Close()

	fmt.Printf("\n%s[Authorization Confirmed]%s\n", colorGreen, colorReset)

	var finalReply SwarmReply
	finalReply.StepFailures = make([]int32, len(activeScenario.Steps))

	if *runMode == "standalone" {
		fmt.Printf("%s[INITIATING LOCAL SWARM]%s\n\n", colorCyan, colorReset)
		args := SwarmArgs{TargetURL: *targetURL, TotalReqs: *totalRequests, TargetRPS: *targetRPS, ScenarioData: activeScenario}
		
		engine := new(WorkerNode)
		engine.ExecuteSwarm(args, &finalReply)
	}

	if *runMode == "master" {
		workers := strings.Split(*workerNodes, ",")
		reqsPerWorker := *totalRequests / len(workers)
		rpsPerWorker := *targetRPS / len(workers)

		fmt.Printf("%s[MASTER NODE COORDINATING] %d WORKERS...%s\n", colorYellow, len(workers), colorReset)
		fmt.Printf("Distributing: %d Reqs / %d RPS per worker node.\n\n", reqsPerWorker, rpsPerWorker)

		var masterWg sync.WaitGroup
		var mu sync.Mutex 

		startTime := time.Now()

		for _, address := range workers {
			masterWg.Add(1)
			go func(addr string) {
				defer masterWg.Done()
				client, err := rpc.Dial("tcp", addr)
				if err != nil {
					fmt.Printf("%s[ERROR] Could not connect to worker at %s%s\n", colorRed, addr, colorReset)
					return
				}

				args := SwarmArgs{TargetURL: *targetURL, TotalReqs: reqsPerWorker, TargetRPS: rpsPerWorker, ScenarioData: activeScenario}
				var reply SwarmReply

				fmt.Printf("[Dispatching attack] Worker %s...\n", addr)
				err = client.Call("WorkerNode.ExecuteSwarm", args, &reply) 
				if err != nil {
					fmt.Printf("%s[ERROR] Worker %s failed: %v%s\n", colorRed, addr, err, colorReset)
					return
				}
				fmt.Printf("[Worker reported back] %s successfully!\n", addr)

				// lock thread before merging arrays
				mu.Lock()
				finalReply.SuccessCount += reply.SuccessCount
				finalReply.FailCount += reply.FailCount
				for i := 0; i < 10000; i++ { finalReply.LatencyHistogram[i] += reply.LatencyHistogram[i] }
				for i := 0; i < 600; i++ { finalReply.StatusHistogram[i] += reply.StatusHistogram[i] }
				for i := range finalReply.StepFailures { finalReply.StepFailures[i] += reply.StepFailures[i] }
				mu.Unlock()
			}(address)
		}

		masterWg.Wait()
		finalReply.Duration = time.Since(startTime)
	}

	var p50, p95, p99 int
	if finalReply.SuccessCount > 0 {
		target50 := int32(float64(finalReply.SuccessCount) * 0.50)
		target95 := int32(float64(finalReply.SuccessCount) * 0.95)
		target99 := int32(float64(finalReply.SuccessCount) * 0.99)
		var runningCount int32 = 0
		for ms, count := range finalReply.LatencyHistogram {
			if count == 0 { continue }
			runningCount += count
			if p50 == 0 && runningCount >= target50 { p50 = ms }
			if p95 == 0 && runningCount >= target95 { p95 = ms }
			if p99 == 0 && runningCount >= target99 { p99 = ms }
		}
	}

	fmt.Printf("\n\n%s==================================================%s\n", colorCyan, colorReset)
	fmt.Printf("[DISTRIBUTED TEST COMPLETE]\n")
	fmt.Printf("Time Elapsed:  %v\n", finalReply.Duration)
	fmt.Printf("Scenario:      %s\n\n", activeScenario.Name)
	fmt.Printf("%sSuccessful:%s    %d\n", colorGreen, colorReset, finalReply.SuccessCount)
	fmt.Printf("%sFailed:%s        %d\n\n", colorRed, colorReset, finalReply.FailCount)

	fmt.Printf("[STEP-BY-STEP FAILURE BREAKDOWN]\n")
	for i, step := range activeScenario.Steps {
		fails := finalReply.StepFailures[i]
		if fails > 0 {
			fmt.Printf("  [X] Step %d [%s %s]: %s%d failures%s\n", i+1, step.Method, step.URL, colorRed, fails, colorReset)
		} else {
			fmt.Printf("  [OK] Step %d [%s %s]: 0 failures\n", i+1, step.Method, step.URL)
		}
	}

	fmt.Printf("\n[HTTP STATUS CODES]\n")
	for code, count := range finalReply.StatusHistogram {
		if count > 0 {
			if code >= 200 && code < 300 {
				fmt.Printf("  %s[%d]: %d%s\n", colorGreen, code, count, colorReset)
			} else {
				fmt.Printf("  %s[%d]: %d%s\n", colorYellow, code, count, colorReset)
			}
		}
	}

	fmt.Printf("\n[LATENCY PERCENTILES] (Full Journey)\n")
	fmt.Printf("  P50 (Median):  %d ms\n", p50)
	fmt.Printf("  P95:           %d ms\n", p95)
	fmt.Printf("  P99:           %d ms\n", p99)
	fmt.Printf("%s==================================================%s\n", colorCyan, colorReset)
}