package udp

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
)

func Client(wg *sync.WaitGroup) {
	// Create UDP address
	serverAddr, err := net.ResolveUDPAddr("udp", "localhost:8081")
	if err != nil {
		fmt.Println("Error resolving address:", err)
		return
	}

	// Create UDP connection
	conn, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected to UDP server. Type your message (exit to quit):")

	// Start goroutine to receive responses
	go func() {
		buffer := make([]byte, 1024)
		for {
			n, _, err := conn.ReadFromUDP(buffer)
			if err != nil {
				fmt.Println("Error reading from server:", err)
				return
			}
			fmt.Printf("Server: %s\n", string(buffer[:n]))
		}
	}()

	// Read and send user input
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		if message == "exit" {
			return
		}

		_, err := conn.Write([]byte(message))
		if err != nil {
			fmt.Println("Error sending message:", err)
			return
		}
	}
}
