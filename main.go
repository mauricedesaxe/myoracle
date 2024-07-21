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

	if *link == "" {
		panic("link is required")
	}
	runNode(*link)
}

func runNode(link string) {
	// do an initial sync to get the nodes
	nodes, err := requestNodes(link)
	if err != nil {
		panic("error first syncing nodes: " + err.Error())
	}
	fmt.Println("First synced to:", len(nodes), "nodes")

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
