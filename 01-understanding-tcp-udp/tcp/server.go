package tcp

import (
	"bufio"
	"fmt"
	"net"
	"sync"
)

func Server(wg *sync.WaitGroup) {
	// Start server
	listener, err := net.Listen("tcp", ":8080")
	if err != nil {
		fmt.Println("Error starting server:", err)
		return
	}
	defer listener.Close()

	fmt.Println("TCP Server listening on :8080")

	for {
		// Accept connections
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("Error accepting connection:", err)
			continue
		}

		// Handle each client in a goroutine
		go handleConnection(conn)
	}
}

func handleConnection(conn net.Conn) {
	defer conn.Close()
	fmt.Printf("New client connected: %s\n", conn.RemoteAddr())

	reader := bufio.NewReader(conn)
	for {
		// Read incoming message
		message, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Client %s disconnected\n", conn.RemoteAddr())
			return
		}

		fmt.Printf("Received from %s: %s", conn.RemoteAddr(), message)

		// Echo message back to client
		conn.Write([]byte("Echo: " + message))
	}
}
