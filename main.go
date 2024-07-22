package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"time"
)

var asset = "ETH"
var denomination = "USD"

var coinGecko = "https://api.coingecko.com/api/v3/coins/markets?vs_currency=" + denomination +
	"&ids=" + asset + "&order=market_cap_by_total_volume&per_page=100&page=1&sparkline=false&price_change_percentage=24h&locale=en"
var coinMarketCap = "https://pro-api.coinmarketcap.com/v1/cryptocurrency/quotes/latest?symbol=" + asset + "&convert=" + denomination

func main() {
	link := flag.String("link", "", "The url of the first node to sync to, will be used to sync the other nodes")
	flag.Parse()

	runNode(*link)
}

func runNode(link string) {
	var err error

	// do an initial sync to get the nodes
	var nodes []string
	if link != "" {
		log.Println("Syncing to:", link)
		nodes, err = requestNodes(link)
		if err != nil {
			panic("error first syncing nodes: " + err.Error())
		}
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

	// Start the HTTP server to communicate with other nodes
	go func() {
		log.Println("Starting HTTP server on :3000")
		if err := http.ListenAndServe(":3000", nil); err != nil {
			log.Fatalf("HTTP server failed: %s", err)
		}
	}()

	// TODO the idea here is that nodes talk to each other to see if they are
	// in a round. If they aren't in a round, every X seconds one randomly elected node
	// will start a round. If a node starts a round, all of them try to get the price
	// from CoinGecko and CoinMarketCap and then calculate the median of medians. If the median is
	// above or below the last median, they update the last median and the leaderboard.
	for {
		time.Sleep(3 * time.Second)

		price1, err := getPriceFromCoinGecko()
		if err != nil {
			log.Println("error getting price from coin gecko: " + err.Error())
		}

		price2, err := getPriceFromCoinMarketCap()
		if err != nil {
			log.Println("error getting price from coin market cap: " + err.Error())
		}

		median := (price1 + price2) / 2
		fmt.Println("Median price:", median)
	}
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

func getPriceFromCoinGecko() (float64, error) {
	resp, err := http.Get(coinGecko)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	type Response struct {
		Price float64 `json:"price"`
	}
	var response Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return 0, err
	}
	return response.Price, nil
}

func getPriceFromCoinMarketCap() (float64, error) {
	resp, err := http.Get(coinMarketCap)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	type Response struct {
		Price float64 `json:"price"`
	}
	var response Response
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return 0, err
	}
	return response.Price, nil
}
