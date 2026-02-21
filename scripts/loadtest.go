//go:build ignore

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"sync/atomic"
	"time"
)

const (
	baseURL  = "http://localhost:8080"
	baseLat  = 12.9716
	baseLng  = 77.5946
)

type Stats struct {
	TotalRequests   int64
	SuccessRequests int64
	FailedRequests  int64
	TotalLatency    int64
	MinLatency      int64
	MaxLatency      int64
}

func main() {
	rand.Seed(time.Now().UnixNano())

	fmt.Println("GoComet Load Test")
	fmt.Println("=================")

	// First, seed some data
	fmt.Println("\n1. Creating test data...")
	userIDs, driverIDs := createTestData()

	if len(userIDs) == 0 || len(driverIDs) == 0 {
		log.Fatal("Failed to create test data")
	}

	fmt.Printf("Created %d users and %d drivers\n", len(userIDs), len(driverIDs))

	// Test 1: Location Update Throughput
	fmt.Println("\n2. Testing Location Updates (1000 updates, 50 concurrent)...")
	stats := testLocationUpdates(driverIDs, 1000, 50)
	printStats("Location Updates", stats)

	// Test 2: Ride Creation
	fmt.Println("\n3. Testing Ride Creation (100 rides, 10 concurrent)...")
	stats = testRideCreation(userIDs, 100, 10)
	printStats("Ride Creation", stats)

	// Test 3: Mixed Load
	fmt.Println("\n4. Testing Mixed Load (30 seconds)...")
	stats = testMixedLoad(userIDs, driverIDs, 30*time.Second)
	printStats("Mixed Load", stats)

	fmt.Println("\nLoad test completed!")
}

func createTestData() ([]string, []string) {
	userIDs := make([]string, 0)
	driverIDs := make([]string, 0)

	// Create users
	for i := 0; i < 20; i++ {
		user := map[string]string{
			"phone": fmt.Sprintf("98%08d", rand.Intn(100000000)),
			"name":  fmt.Sprintf("LoadTest User %d", i),
		}
		body, _ := json.Marshal(user)
		resp, err := http.Post(baseURL+"/v1/users", "application/json", bytes.NewBuffer(body))
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 201 {
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			if id, ok := result["id"].(string); ok {
				userIDs = append(userIDs, id)
			}
		}
	}

	// Create drivers
	vehicleTypes := []string{"auto", "mini", "sedan", "suv"}
	for i := 0; i < 50; i++ {
		vt := vehicleTypes[rand.Intn(len(vehicleTypes))]
		driver := map[string]string{
			"phone":          fmt.Sprintf("91%08d", rand.Intn(100000000)),
			"name":           fmt.Sprintf("LoadTest Driver %d", i),
			"license_number": fmt.Sprintf("DL%07d", rand.Intn(10000000)),
			"vehicle_type":   vt,
			"vehicle_number": fmt.Sprintf("KA%02dAB%04d", rand.Intn(99), rand.Intn(10000)),
		}
		body, _ := json.Marshal(driver)
		resp, err := http.Post(baseURL+"/v1/drivers", "application/json", bytes.NewBuffer(body))
		if err != nil {
			continue
		}
		defer resp.Body.Close()

		if resp.StatusCode == 201 {
			var result map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&result)
			if id, ok := result["id"].(string); ok {
				driverIDs = append(driverIDs, id)

				// Set driver online
				http.Post(baseURL+"/v1/drivers/"+id+"/online", "application/json", nil)
			}
		}
	}

	return userIDs, driverIDs
}

func testLocationUpdates(driverIDs []string, numRequests, concurrency int) *Stats {
	stats := &Stats{MinLatency: int64(^uint64(0) >> 1)}
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(driverID string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			lat := baseLat + (rand.Float64()-0.5)*0.1
			lng := baseLng + (rand.Float64()-0.5)*0.1

			payload := map[string]float64{
				"lat": lat,
				"lng": lng,
			}
			body, _ := json.Marshal(payload)

			start := time.Now()
			resp, err := http.Post(baseURL+"/v1/drivers/"+driverID+"/location", "application/json", bytes.NewBuffer(body))
			latency := time.Since(start).Milliseconds()

			atomic.AddInt64(&stats.TotalRequests, 1)
			atomic.AddInt64(&stats.TotalLatency, latency)

			if err != nil || resp.StatusCode != 200 {
				atomic.AddInt64(&stats.FailedRequests, 1)
				if resp != nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			atomic.AddInt64(&stats.SuccessRequests, 1)

			for {
				old := atomic.LoadInt64(&stats.MinLatency)
				if latency >= old || atomic.CompareAndSwapInt64(&stats.MinLatency, old, latency) {
					break
				}
			}
			for {
				old := atomic.LoadInt64(&stats.MaxLatency)
				if latency <= old || atomic.CompareAndSwapInt64(&stats.MaxLatency, old, latency) {
					break
				}
			}
		}(driverIDs[rand.Intn(len(driverIDs))])
	}

	wg.Wait()
	return stats
}

func testRideCreation(userIDs []string, numRequests, concurrency int) *Stats {
	stats := &Stats{MinLatency: int64(^uint64(0) >> 1)}
	var wg sync.WaitGroup
	semaphore := make(chan struct{}, concurrency)

	for i := 0; i < numRequests; i++ {
		wg.Add(1)
		semaphore <- struct{}{}

		go func(idx int, userID string) {
			defer wg.Done()
			defer func() { <-semaphore }()

			ride := map[string]interface{}{
				"user_id": userID,
				"pickup": map[string]float64{
					"lat": baseLat + (rand.Float64()-0.5)*0.1,
					"lng": baseLng + (rand.Float64()-0.5)*0.1,
				},
				"dropoff": map[string]float64{
					"lat": baseLat + (rand.Float64()-0.5)*0.1,
					"lng": baseLng + (rand.Float64()-0.5)*0.1,
				},
				"vehicle_type":   "sedan",
				"payment_method": "cash",
			}
			body, _ := json.Marshal(ride)

			req, _ := http.NewRequest("POST", baseURL+"/v1/rides", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Idempotency-Key", fmt.Sprintf("load-test-ride-%d-%d", idx, time.Now().UnixNano()))

			start := time.Now()
			resp, err := http.DefaultClient.Do(req)
			latency := time.Since(start).Milliseconds()

			atomic.AddInt64(&stats.TotalRequests, 1)
			atomic.AddInt64(&stats.TotalLatency, latency)

			if err != nil || (resp.StatusCode != 201 && resp.StatusCode != 409) {
				atomic.AddInt64(&stats.FailedRequests, 1)
				if resp != nil {
					io.Copy(io.Discard, resp.Body)
					resp.Body.Close()
				}
				return
			}
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()

			atomic.AddInt64(&stats.SuccessRequests, 1)

			for {
				old := atomic.LoadInt64(&stats.MinLatency)
				if latency >= old || atomic.CompareAndSwapInt64(&stats.MinLatency, old, latency) {
					break
				}
			}
			for {
				old := atomic.LoadInt64(&stats.MaxLatency)
				if latency <= old || atomic.CompareAndSwapInt64(&stats.MaxLatency, old, latency) {
					break
				}
			}
		}(i, userIDs[rand.Intn(len(userIDs))])
	}

	wg.Wait()
	return stats
}

func testMixedLoad(userIDs, driverIDs []string, duration time.Duration) *Stats {
	stats := &Stats{MinLatency: int64(^uint64(0) >> 1)}
	var wg sync.WaitGroup
	done := make(chan struct{})

	// Location update workers (high frequency)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-done:
					return
				default:
					driverID := driverIDs[rand.Intn(len(driverIDs))]
					lat := baseLat + (rand.Float64()-0.5)*0.1
					lng := baseLng + (rand.Float64()-0.5)*0.1

					payload := map[string]float64{"lat": lat, "lng": lng}
					body, _ := json.Marshal(payload)

					start := time.Now()
					resp, err := http.Post(baseURL+"/v1/drivers/"+driverID+"/location", "application/json", bytes.NewBuffer(body))
					latency := time.Since(start).Milliseconds()

					atomic.AddInt64(&stats.TotalRequests, 1)
					atomic.AddInt64(&stats.TotalLatency, latency)

					if err != nil || resp.StatusCode != 200 {
						atomic.AddInt64(&stats.FailedRequests, 1)
					} else {
						atomic.AddInt64(&stats.SuccessRequests, 1)
					}

					if resp != nil {
						io.Copy(io.Discard, resp.Body)
						resp.Body.Close()
					}

					time.Sleep(10 * time.Millisecond)
				}
			}
		}()
	}

	// Wait for duration
	time.Sleep(duration)
	close(done)
	wg.Wait()

	return stats
}

func printStats(name string, stats *Stats) {
	avgLatency := float64(0)
	if stats.TotalRequests > 0 {
		avgLatency = float64(stats.TotalLatency) / float64(stats.TotalRequests)
	}

	fmt.Printf("\n%s Results:\n", name)
	fmt.Printf("  Total Requests:   %d\n", stats.TotalRequests)
	fmt.Printf("  Successful:       %d\n", stats.SuccessRequests)
	fmt.Printf("  Failed:           %d\n", stats.FailedRequests)
	fmt.Printf("  Success Rate:     %.2f%%\n", float64(stats.SuccessRequests)/float64(stats.TotalRequests)*100)
	fmt.Printf("  Avg Latency:      %.2f ms\n", avgLatency)
	if stats.MinLatency != int64(^uint64(0)>>1) {
		fmt.Printf("  Min Latency:      %d ms\n", stats.MinLatency)
	}
	fmt.Printf("  Max Latency:      %d ms\n", stats.MaxLatency)
	fmt.Printf("  Throughput:       %.0f req/s\n", float64(stats.TotalRequests)/(float64(stats.TotalLatency)/1000))
}
