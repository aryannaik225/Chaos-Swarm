# Chaos Swarm 🌪️
**An Enterprise-Grade Distributed Load Testing & Orchestration Engine**

**⚠️ LEGAL DISCLAIMER & ETHICAL USE POLICY**
> This software was engineered strictly for educational purposes, academic research, and authorized quality assurance testing. **It is a fundamental violation of this project's intent to use it against any server or network infrastructure without explicit, prior, written consent from the infrastructure owners.** > The author (Aryan) holds absolute zero liability for any misuse, damage, or legal repercussions caused by the utilization of this codebase. To enforce ethical use, this engine features a cryptographic handshake lock and will refuse to execute an attack unless authorized domain ownership is mathematically verified via a target-side authentication file.

---

## 📌 Executive Summary (For Recruiters & Product Managers)
Chaos Swarm is a high-performance HTTP load-testing engine built entirely from scratch in Go. While traditional tools struggle to simulate complex user behavior at scale without crashing the host machine, Chaos Swarm utilizes distributed network orchestration and lock-free memory management to generate massive, stateful traffic. 

It doesn't just hit a single URL; it reads a JSON script, spins up thousands of virtual users, logs them in, manages their session cookies, and tracks exactly where the server fails—all while calculating precise P99 latency percentiles.

---

## 🧠 System Architecture & Engineering Deep Dive (For Senior Engineers)
Why build another load tester when JMeter and Locust exist? To understand how to break the physical limits of the OS.

<div align="center">
  <img src="https://github.com/aryannaik225/Chaos-Swarm/blob/main/images/network_topology.png" alt="Mater Node -> Multiple Worker Nodes -> Target Server" width="550" />
  <p>The Network Topology</p>
</div>

### 1. Distributed RPC Orchestration (Bypassing OS Socket Limits)
A single Linux machine is bound by its ephemeral port limit (~65,000 ports) and TCP Accept Queue bottlenecks. To achieve enterprise-scale RPS (Requests Per Second), Chaos Swarm uses Go's `net/rpc` to operate in a distributed cluster.
* **Master Node:** Parses the attack scenario, splits the load, and dispatches instructions over TCP to the fleet.
* **Worker Nodes:** Headless network assassins that receive instructions, execute the localized attack using a shared `http.Transport` connection pool, and stream the telemetry back to the Master.

### 2. Lock-Free Atomic Telemetry
Tracking latencies for 100,000+ concurrent requests usually requires Mutex locks, which creates massive CPU contention and slows down the tester itself. Chaos Swarm solves this using **Lock-Free HDR Histograms**.
By pre-allocating static memory arrays `[10000]int32` and utilizing `sync/atomic` counters, multiple Goroutines can concurrently log their latencies and HTTP status codes at the nanosecond level with zero blocking.

### 3. Stateful Protocol Journeys
Real users don't just hit `/api/data`. They log in, get a token, and use that token to interact with the system. Chaos Swarm parses `scenario.json` files to execute multi-step API journeys. Each Goroutine is equipped with its own isolated `http.CookieJar`, allowing it to retain session state across complex authentication flows just like a real browser.

---

## 🚀 Installation & Quick Start

### Prerequisites
* Go 1.20 or higher installed.

### Setting the Security Lock
Before running, you must authorize the target server. Generate a token, create a `swarm-auth.txt` file, and place it in the root directory of your target server.
```bash
echo "SWARM-your_token_here" > swarm-auth.txt
```

---

## 🎮 Usage Modes

<div align="center">
  <img src="https://github.com/aryannaik225/Chaos-Swarm/blob/main/images/bot_lifestyle.png" alt="Read JSON Scenario -> POST /login -> GET /cart -> Report to Atomic Histogram" width="650" />
  <p>The Stateful Journey</p>
</div>

### Mode 1: Standalone (Solo Developer Mode)
Perfect for quick, single-machine API testing. Runs the entire swarm engine locally in your terminal with a live UI ticker.

```bash
go run cmd/swarm/main.go --scenario=scenario.json --requests=10000 --rps=1000
```

### Mode 2: Distributed Fleet (Enterprise Scale)
Deploy the workers on separate physical servers or terminal instances to multiply your network bandwidth.

**Step 1: Spin up the Workers**
```bash
# Terminal 1
go run cmd/swarm/main.go --mode=worker --port=:9001

# Terminal 2
go run cmd/swarm/main.go --mode=worker --port=:9002
```

**Step 2: Command the Master**
```bash
go run cmd/swarm/main.go --mode=master --workers=localhost:9001,localhost:9002 --scenario=scenario.json --requests=50000 --rps=5000
```

---

## 📊 Defining a Scenario (`scenario.json`)
The engine reads JSON to understand the exact journey you want the virtual users to take.

```json
{
  "name": "VIP Checkout Flow",
  "steps": [
    {
      "method": "POST",
      "url": "http://localhost:8080/api/login",
      "payload": "{\"email\":\"bot@swarm.com\",\"password\":\"hacktheplanet\"}"
    },
    {
      "method": "GET",
      "url": "http://localhost:8080/api/cart",
      "payload": ""
    },
    {
      "method": "POST",
      "url": "http://localhost:8080/api/checkout",
      "payload": "{\"ticketId\":\"VIP-13\"}"
    }
  ]
}
```

---

## 📡 Observability & Telemetry Output
Upon completion, the engine aggregates data from all nodes to provide a granular telemetry breakdown:
1. **Step-by-Step Failure Breakdown:** Pinpoints exactly which step of the JSON scenario broke the server.
2. **HTTP Status Distribution:** Counts every 2xx, 4xx, and 5xx error.
3. **Latency Percentiles (P50, P95, P99):** Uses the atomic histograms to calculate the exact millisecond latency of the worst-case network requests.

<div align="center">
  <img src="https://github.com/aryannaik225/Chaos-Swarm/blob/main/images/terminal_output.png" alt="Terminal Output Showing P99 Latencies" width="800" />
</div>

---

## 📄 License
This project is licensed under the MIT License - see the [LICENSE](https://github.com/aryannaik225/Chaos-Swarm/blob/main/LICENSE) for details.
