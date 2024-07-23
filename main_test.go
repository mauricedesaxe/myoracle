package main

import (
	"testing"
	"time"
)

func TestMain(t *testing.T) {
	// Run the first node
	go runNode(NodeConfig{
		Link:          "",
		BaseUrl:       "http://localhost",
		Port:          ":3000",
		DiffThreshold: 0.01,
		TimeInterval:  1,
	})

	time.Sleep(1 * time.Second)

	// Run the second node
	go runNode(NodeConfig{
		Link:          "http://localhost:3000",
		BaseUrl:       "http://localhost",
		Port:          ":3001",
		DiffThreshold: 0.01,
		TimeInterval:  1,
	})

	time.Sleep(1 * time.Second)

	// Run the third node
	go runNode(NodeConfig{
		Link:          "http://localhost:3001",
		BaseUrl:       "http://localhost",
		Port:          ":3002",
		DiffThreshold: 0.01,
		TimeInterval:  1,
	})

	// Allow some time for nodes to communicate
	time.Sleep(10 * time.Second)
}
