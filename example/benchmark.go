package main

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

func main() {
	url := "http://localhost:8080/ping"
	duration := 10 * time.Second
	concurrency := 200 // Heavy load

	fmt.Printf("🔥 Starting HEAVY Load Test on %s\n", url)
	fmt.Printf("   Concurrency: %d workers\n", concurrency)
	fmt.Printf("   Duration:    %s\n", duration)

	start := time.Now()
	endCh := make(chan struct{})

	// Latency collection
	var latencies []time.Duration
	var mu sync.Mutex

	var wg sync.WaitGroup
	wg.Add(concurrency)

	// Create workers
	for i := 0; i < concurrency; i++ {
		go func() {
			defer wg.Done()
			client := &http.Client{
				Transport: &http.Transport{
					MaxIdleConns:        500,
					MaxIdleConnsPerHost: 500,
					IdleConnTimeout:     30 * time.Second,
				},
				Timeout: 5 * time.Second,
			}

			localLatencies := make([]time.Duration, 0, 1000)

			for {
				select {
				case <-endCh:
					// Flush local stats
					mu.Lock()
					latencies = append(latencies, localLatencies...)
					mu.Unlock()
					return
				default:
					reqStart := time.Now()
					resp, err := client.Get(url)
					if err != nil || resp.StatusCode != 200 {
						// Ignore errors for max throughput loop
					} else {
						resp.Body.Close()
						localLatencies = append(localLatencies, time.Since(reqStart))
					}
				}
			}
		}()
	}

	// Wait loop
	time.Sleep(duration)
	close(endCh)
	wg.Wait()

	totalTime := time.Since(start).Seconds()

	// Analyze
	count := len(latencies)
	if count == 0 {
		fmt.Println("No successful requests.")
		return
	}

	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })

	var sum time.Duration
	for _, l := range latencies {
		sum += l
	}
	avg := sum / time.Duration(count)

	fmt.Println("\n------------------------------------------------")
	fmt.Printf("✅ Requests: %d\n", count)
	fmt.Printf("⏱  Total Time: %.2fs\n", totalTime)
	fmt.Printf("🚀 RPS:      %.2f req/sec\n", float64(count)/totalTime)
	fmt.Println("------------------------------------------------")
	fmt.Println("📊 Latency Distribution:")
	fmt.Printf("   Avg: %v\n", avg)
	fmt.Printf("   Min: %v\n", latencies[0])
	fmt.Printf("   P50: %v\n", latencies[count*50/100])
	fmt.Printf("   P95: %v\n", latencies[count*95/100])
	fmt.Printf("   P99: %v\n", latencies[count*99/100])
	fmt.Printf("   Max: %v\n", latencies[count-1])
	fmt.Println("------------------------------------------------")
}
