package tcp

import (
	"bufio"
	"fmt"
	"net"
	"os"
	"sync"
)

func Client(wg *sync.WaitGroup) {
	// Connect to server
	conn, err := net.Dial("tcp", "localhost:8080")
	if err != nil {
		fmt.Println("Error connecting:", err)
		return
	}
	defer conn.Close()

	fmt.Println("Connected to server. Type your message (exit to quit):")

	// Start a goroutine to read server responses
	go func() {
		reader := bufio.NewReader(conn)
		for {
			message, err := reader.ReadString('\n')
			if err != nil {
				fmt.Println("Server connection closed")
				os.Exit(0)
			}
			fmt.Print("Server: ", message)
		}
	}()

	// Read user input and send to server
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		message := scanner.Text()
		if message == "exit" {
			return
		}
		fmt.Fprintf(conn, "%s\n", message)
	}
}
