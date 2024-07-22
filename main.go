package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"time"
)

type NodeConfig struct {
	Link          string
	BaseUrl       string
	Port          string
	DiffThreshold float64
}

func main() {
	link := flag.String("link", "", "The url of the first node to sync to, will be used to sync the other nodes")
	baseUrl := flag.String("baseUrl", "http://localhost", "The base url of the node")
	port := flag.String("port", ":3000", "The port of the node")
	diffThreshold := flag.Float64("diffThreshold", 0.01, "The threshold for the median to change by")
	flag.Parse()

	runNode(NodeConfig{
		Link:          *link,
		BaseUrl:       *baseUrl,
		Port:          *port,
		DiffThreshold: *diffThreshold,
	})
}

func runNode(config NodeConfig) {
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
	http.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		json.NewEncoder(w).Encode(nodes)
	})

	var isRound bool
	var lastMedian float64

	// POST /median - receives the median from the leader, and sends back a median if the req is valid
	http.HandleFunc("/median", func(w http.ResponseWriter, r *http.Request) {
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

		// if median from leader is valid, start round and provide a median back
		log.Println(time.Now().Format("2006-01-02 15:04:05"), "Received median:", request.Median)
		isRound = true
		_, err = http.Post(
			config.BaseUrl+config.Port+"/median",
			"application/json",
			bytes.NewBuffer([]byte(fmt.Sprintf(`{"median": %f}`, request.Median))),
		)
		if err != nil {
			log.Println("Error sending median:", err)
		}
		isRound = false
	})

	// Try to start a round every 10 seconds as a leader
	go func() {
		for {
			if isRound {
				continue
			}
			isRound = true

			median := getFakeMedian()

			diff := median - lastMedian
			relDiff := diff / lastMedian
			if relDiff < config.DiffThreshold {
				continue
			}
			log.Println("Median changed by more than", config.DiffThreshold*100, "%, sending to nodes")

			// send the median to all nodes
			log.Println(time.Now().Format("2006-01-02 15:04:05"), "Sending median:", median)
			for _, node := range nodes {
				http.Post(
					node+"/median",
					"application/json",
					bytes.NewBuffer([]byte(fmt.Sprintf(`{"median": %f}`, median))),
				)
			}

			lastMedian = median
			isRound = false
			time.Sleep(10 * time.Second)
		}
	}()

	// Start the HTTP server to communicate with other nodes
	go func() {
		log.Println("Starting HTTP server on " + fmt.Sprintf("%s%s", config.BaseUrl, config.Port))
		if err := http.ListenAndServe(config.Port, nil); err != nil {
			log.Fatalf("HTTP server failed: %s", err)
		}
	}()
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
	type Response struct {
		Nodes []string `json:"nodes"`
	}
	var response Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return nil, err
	}
	return response.Nodes, nil
}
