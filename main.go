package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"sync"
	"time"
)

type NodeConfig struct {
	Link          string
	BaseUrl       string
	Port          string
	DiffThreshold float64
	TimeInterval  int
}

func main() {
	link := flag.String("link", "", "The url of the first node to sync to, will be used to sync the other nodes")
	baseUrl := flag.String("baseUrl", "http://localhost", "The base url of the node")
	port := flag.String("port", ":3000", "The port of the node")
	diffThreshold := flag.Float64("diffThreshold", 0.01, "The threshold for the median to change by")
	timeInterval := flag.Int("timeInterval", 10, "The time interval in seconds to wait before sending a new median")
	flag.Parse()

	runNode(NodeConfig{
		Link:          *link,
		BaseUrl:       *baseUrl,
		Port:          *port,
		DiffThreshold: *diffThreshold,
		TimeInterval:  *timeInterval,
	})
}

func runNode(config NodeConfig) {
	mux := http.NewServeMux()

	// do an initial sync to get the nodes
	nodes := []string{
		config.BaseUrl + config.Port,
	}
	if config.Link != "" {
		log.Println("Syncing to:", config.Link)
		n, err := requestNodes(config.Link)
		if err != nil {
			panic("error first syncing nodes: " + err.Error())
		}
		nodes = append(nodes, n...)
		log.Println("Synced to:", len(nodes), "nodes")
	}

	// POST /sync - returns a list of nodes
	mux.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		log.Println(time.Now().Format("2006-01-02 15:04:05"), "Sync request from:", r.RemoteAddr)
		json.NewEncoder(w).Encode(nodes)
	})

	var isRound bool
	var lastMedian float64
	var answers []float64
	var mu sync.Mutex

	// POST /median - receives the median from the leader, and sends back a median if the req is valid
	mux.HandleFunc("/median", func(w http.ResponseWriter, r *http.Request) {
		// only allow POST requests
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// decode median
		type Request struct {
			Median float64 `json:"median"`
		}
		var request Request
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		mu.Lock()
		defer mu.Unlock()

		// If a round is already in process (i.e. this is a median response and we are the leader)
		// then we need to check if we have enough answers to calculate a final answer.
		// If we do we "push" a final answer, if not we store this new median and wait
		// for more medians to arrive.
		if isRound {
			if len(nodes) < 3 {
				log.Println("not enough nodes to complete a round")
				return
			}

			if len(answers) >= len(nodes)/3*2 { // 2/3 of the nodes need to respond
				log.Println("Round complete", answers)
				answers = []float64{}
				isRound = false
				return
			}

			answers = append(answers, request.Median)
			return
		}

		// if median from leader is valid, start round and provide a median back
		log.Println(time.Now().Format("2006-01-02 15:04:05"), "Received median from:", r.RemoteAddr, "value:", request.Median)
		isRound = true
		_, err = http.Post(
			config.BaseUrl+config.Port+"/median",
			"application/json",
			bytes.NewBuffer([]byte(fmt.Sprintf(`{"median": %f}`, getFakeMedian()))),
		)
		if err != nil {
			log.Println("Error sending median:", err)
		}
		isRound = false
	})

	// Try to start a round every 10 seconds as a leader
	go func() {
		for {
			mu.Lock()
			if isRound {
				mu.Unlock()
				continue
			}
			isRound = true
			mu.Unlock()

			median := getFakeMedian()

			diff := median - lastMedian
			relDiff := diff / lastMedian
			if relDiff < config.DiffThreshold {
				mu.Lock()
				isRound = false
				mu.Unlock()
				continue
			}
			log.Println("Median changed by more than", config.DiffThreshold*100, "%, sending to nodes")

			// send the median to all nodes
			log.Println(time.Now().Format("2006-01-02 15:04:05"), "Sending median:", median)
			for _, node := range nodes {
				if node == config.BaseUrl+config.Port {
					continue
				}
				http.Post(
					node+"/median",
					"application/json",
					bytes.NewBuffer([]byte(fmt.Sprintf(`{"median": %f}`, median))),
				)
			}

			lastMedian = median
			mu.Lock()
			isRound = false
			mu.Unlock()
			time.Sleep(time.Duration(config.TimeInterval) * time.Second)
		}
	}()

	// Start the HTTP server to communicate with other nodes
	log.Println("Starting HTTP server on " + fmt.Sprintf("%s%s", config.BaseUrl, config.Port))
	if err := http.ListenAndServe(config.Port, mux); err != nil {
		log.Fatalf("HTTP server failed: %s", err)
	}
}

func getFakeMedian() float64 {
	price1 := 999 + rand.Float64()*2
	price2 := 999 + rand.Float64()*2
	price3 := 999 + rand.Float64()*2
	median := (price1 + price2 + price3) / 3
	return median
}

func requestNodes(node string) ([]string, error) {
	req, err := http.NewRequest("POST", node+"/sync", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var nodes []string
	err = json.NewDecoder(resp.Body).Decode(&nodes)
	if err != nil {
		return nil, err
	}
	return nodes, nil
}
