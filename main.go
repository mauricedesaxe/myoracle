package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
)

func main() {
	link := flag.String("link", "", "The url of the first node to sync to, will be used to sync the other nodes")
	flag.Parse()

	// do an initial sync to get the nodes
	nodes, err := requestNodes(*link)
	if err != nil {
		panic("error first syncing nodes: " + err.Error())
	}
	fmt.Println("First synced to:", len(nodes), "nodes")
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
