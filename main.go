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

var asset = "ETH"
var denomination = "USD"

var coinGecko = "https://api.coingecko.com/api/v3/coins/markets?vs_currency=" + denomination +
	"&ids=" + asset + "&order=market_cap_by_total_volume&per_page=100&page=1&sparkline=false&price_change_percentage=24h&locale=en"
var coinMarketCap = "https://pro-api.coinmarketcap.com/v1/cryptocurrency/quotes/latest?symbol=" + asset + "&convert=" + denomination

type NodeConfig struct {
	Link    string
	BaseUrl string
	Port    string
}

func main() {
	link := flag.String("link", "", "The url of the first node to sync to, will be used to sync the other nodes")
	baseUrl := flag.String("baseUrl", "http://localhost", "The base url of the node")
	port := flag.String("port", ":3000", "The port of the node")
	flag.Parse()

	runNode(NodeConfig{
		Link:    *link,
		BaseUrl: *baseUrl,
		Port:    *port,
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

	// Start the leader election
	go func() {
		for {
			if isRound {
				continue
			}
			isRound = true

			// fake get data from 3 providers
			price1 := 999 + rand.Float64()*2
			price2 := 999 + rand.Float64()*2
			price3 := 999 + rand.Float64()*2
			median := (price1 + price2 + price3) / 3
			log.Println(time.Now().Format("2006-01-02 15:04:05"), "Median price:", median)
			// send the median to all nodes
			for _, node := range nodes {
				http.Post(node+"/median", "application/json", bytes.NewBuffer([]byte(fmt.Sprintf(`{"median": %f}`, median))))
			}

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
