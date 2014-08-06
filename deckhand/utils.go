package deckhand

import (
	"fmt"
	"net"
	"os"
)

// Get host IP addresses
func getHostIps() ([]string, error) {
	name, err := os.Hostname()
	if err != nil {
		fmt.Printf("Error getting hostname: %v\n", err)
		return nil, err
	}

	addrs, err2 := net.LookupHost(name)
	return addrs, err2
}
