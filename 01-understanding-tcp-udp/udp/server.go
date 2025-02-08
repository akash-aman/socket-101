package udp

import (
	"fmt"
	"net"
	"sync"
)

func Server(wg *sync.WaitGroup) {
	// Create UDP address
	addr, err := net.ResolveUDPAddr("udp", ":8081")
	if err != nil {
		fmt.Println("Error resolving address:", err)
		return
	}

	// Create UDP connection
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		fmt.Println("Error listening:", err)
		return
	}
	defer conn.Close()

	fmt.Println("UDP Server listening on :8081")

	buffer := make([]byte, 1024)
	for {
		// Read incoming message
		n, remoteAddr, err := conn.ReadFromUDP(buffer)
		if err != nil {
			fmt.Println("Error reading from UDP:", err)
			continue
		}

		message := string(buffer[:n])
		fmt.Printf("Received from %s: %s\n", remoteAddr, message)

		// Send response back to client
		response := "Echo: " + message
		_, err = conn.WriteToUDP([]byte(response), remoteAddr)
		if err != nil {
			fmt.Printf("Error sending response to %s: %s\n", remoteAddr, err)
		}
	}
}
