package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/http"
	"sort"
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

func logg(node string, msg string) {
	log.Printf("[%s] %s", node, msg)
}

func runNode(config NodeConfig) {
	mux := http.NewServeMux()

	// do an initial sync to get the nodes
	nodes := []string{
		config.BaseUrl + config.Port,
	}
	if config.Link != "" {
		logg(config.BaseUrl+config.Port, "Syncing to: "+config.Link)
		n, err := syncToNodes(config)
		if err != nil {
			panic("error first syncing nodes: " + err.Error())
		}
		nodes = append(nodes, n...)
		nodes = removeDuplicates(nodes)
		logg(config.BaseUrl+config.Port, "Synced to: "+fmt.Sprint(len(nodes)-1)+" nodes") // subtract 1 because the node itself is included
	}

	mux.HandleFunc("/sync", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// decode node from request
		type Request struct {
			Node string `json:"node"`
		}
		var request Request
		err := json.NewDecoder(r.Body).Decode(&request)
		if err != nil {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		nodes = append(nodes, request.Node)
		nodes = removeDuplicates(nodes)
		logg(config.BaseUrl+config.Port, "Got request to sync from "+request.Node)
		json.NewEncoder(w).Encode(nodes)
	})

	// GET /answer - returns an answer; is to be called by the round leader
	mux.HandleFunc("/answer", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		requestingNode := r.URL.Query().Get("node")
		if requestingNode == "" {
			http.Error(w, "Bad request", http.StatusBadRequest)
			return
		}

		var isValidNode bool
		for _, node := range nodes {
			if node == requestingNode {
				isValidNode = true
				break
			}
		}
		if !isValidNode {
			http.Error(w, "Invalid node", http.StatusBadRequest)
			return
		}
		logg(config.BaseUrl+config.Port, "Got a median request from "+requestingNode)
		json.NewEncoder(w).Encode(getAnswer())
	})

	// Try to start a round every 10 seconds as a leader
	var lastAnswer float64
	var i int
	go func() {
		for {
			if i != 0 {
				time.Sleep(time.Duration(config.TimeInterval) * time.Second)
			}
			i++

			myAnswer := getAnswer()
			diff := myAnswer - lastAnswer
			relDiff := diff / lastAnswer
			if relDiff < config.DiffThreshold {
				continue
			}
			logg(config.BaseUrl+config.Port, "Median changed by more than "+fmt.Sprint(config.DiffThreshold*100)+"%")

			if len(nodes) < 3 {
				logg(config.BaseUrl+config.Port, "not enough nodes to start a round")
				continue
			}

			var answers []float64
			for _, node := range nodes {
				if node == config.BaseUrl+config.Port {
					continue
				}

				logg(config.BaseUrl+config.Port, "Requesting median from "+node)
				resp, err := http.Get(
					node + "/answer?node=" + config.BaseUrl + config.Port,
				)
				if err != nil {
					logg(config.BaseUrl+config.Port, "Error requesting median from "+node+": "+err.Error())
					continue
				}
				var answer float64
				err = json.NewDecoder(resp.Body).Decode(&answer)
				if err != nil {
					logg(config.BaseUrl+config.Port, "Error decoding median from "+node+": "+err.Error())
					continue
				}
				resp.Body.Close()

				answers = append(answers, answer)
			}

			logg(config.BaseUrl+config.Port, "Medians: "+fmt.Sprint(answers))
			newAnswer := getMedian(answers)
			logg(config.BaseUrl+config.Port, "New median: "+fmt.Sprint(newAnswer))
			lastAnswer = newAnswer
		}
	}()

	// Start the HTTP server to communicate with other nodes
	logg(config.BaseUrl+config.Port, "Starting HTTP server on "+fmt.Sprintf("%s%s", config.BaseUrl, config.Port))
	if err := http.ListenAndServe(config.Port, mux); err != nil {
		log.Fatalf("HTTP server failed: %s", err)
	}
}

func getAnswer() float64 {
	price1 := 999 + rand.Float64()*2
	price2 := 999 + rand.Float64()*2
	price3 := 999 + rand.Float64()*2
	median := (price1 + price2 + price3) / 3
	return median
}

// Syncs to "link" node, then requests sync from all received nodes.
// This is important because we need to make sure each node knows about each other node.
// Warning: can return duplicates.
func syncToNodes(config NodeConfig) ([]string, error) {
	reqBody := bytes.NewBuffer([]byte(fmt.Sprintf(`{"node": "%s"}`, config.BaseUrl+config.Port)))
	req, err := http.NewRequest("POST", config.Link+"/sync", reqBody)
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

	// request sync from each other node for redundancy AND to make sure
	// each node knows about me (the new node)
	for _, node := range nodes {
		if node == config.BaseUrl+config.Port {
			continue
		}

		reqBody := bytes.NewBuffer([]byte(fmt.Sprintf(`{"node": "%s"}`, config.BaseUrl+config.Port)))
		req, err := http.NewRequest("POST", node+"/sync", reqBody)
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		var loopNodes []string
		err = json.NewDecoder(resp.Body).Decode(&loopNodes)
		if err != nil {
			return nil, err
		}

		nodes = append(nodes, loopNodes...)
		nodes = removeDuplicates(nodes)
	}

	return nodes, nil
}

func removeDuplicates(nodes []string) []string {
	uniqueNodes := make([]string, 0, len(nodes))
	nodeMap := make(map[string]bool)
	for _, node := range nodes {
		if !nodeMap[node] {
			uniqueNodes = append(uniqueNodes, node)
			nodeMap[node] = true
		}
	}
	return uniqueNodes
}

func getMedian(numbers []float64) float64 {
	sort.Float64s(numbers)
	median := numbers[len(numbers)/2]
	return median
}
