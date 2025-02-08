package tcp

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync"
)

const MaxFrameSize = 65535

type Client struct {
	conn net.Conn
}

type Message struct {
	Type    byte
	Payload []byte
}

func NewClient(wg *sync.WaitGroup) {
	defer wg.Done()
	conn, err := net.Dial("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return
	}

	client := &Client{conn: conn}

	defer client.Close()

	err = client.Handshake()
	if err != nil {
		log.Fatal(err)
	}

	// Read from message.txt file and send.
	message, err := os.ReadFile("tcp/message.txt")
	if err != nil {
		log.Fatalf("Error reading message.txt: %v", err)
	}

	err = client.SendTextMessage(string(message))
	if err != nil {
		log.Fatalf("Error sending message: %v", err)
	}

	// Read messages
	for {
		message, err := client.ReadMessage()
		if err != nil {
			log.Printf("Error reading message: %v", err)
			return
		}
		if message != nil {
			log.Printf("Received message: %s", string(message))
		}
	}
}

func (c *Client) Close() error {
	return c.conn.Close()
}

func (c *Client) Handshake() error {
	// Generate random 16-byte key using crypto/rand
	key := make([]byte, 16)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("error generating random key: %v", err)
	}
	websocketKey := base64.StdEncoding.EncodeToString(key)

	// Send WebSocket handshake request
	handshake := fmt.Sprintf(
		"GET / HTTP/1.1\r\n"+
			"Host: localhost\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Key: %s\r\n"+
			"Sec-WebSocket-Version: 13\r\n"+
			"\r\n",
		websocketKey,
	)

	_, err := c.conn.Write([]byte(handshake))
	if err != nil {
		return fmt.Errorf("error sending handshake: %v", err)
	}

	// Read handshake response
	response := make([]byte, 1024)
	n, err := c.conn.Read(response)
	if err != nil {
		return fmt.Errorf("error reading handshake response: %v", err)
	}

	// Verify the response contains "101 Switching Protocols"
	if string(response[:n])[9:32] != "101 Switching Protocols" {
		return fmt.Errorf("invalid handshake response")
	}

	log.Println("WebSocket handshake completed")
	return nil
}

func (c *Client) sendFrame(frame *Frame) error {
	// Prepare the frame header
	header := make([]byte, 2)
	
	// Set FIN bit based on frame.Fin
	if frame.Fin {
		header[0] = byte(0x80) | frame.Opcode // Set FIN bit + Opcode
	} else {
		header[0] = frame.Opcode // Just Opcode, FIN bit is 0
	}

	// Set payload length and masking bit
	if frame.PayloadLen <= 125 {
		header[1] = byte(frame.PayloadLen) | 0x80 // Set masking bit
	} else if frame.PayloadLen <= 65535 {
		header[1] = 126 | 0x80
		header = append(header, make([]byte, 2)...)
		binary.BigEndian.PutUint16(header[2:], uint16(frame.PayloadLen))
	} else {
		header[1] = 127 | 0x80
		header = append(header, make([]byte, 8)...)
		binary.BigEndian.PutUint64(header[2:], frame.PayloadLen)
	}

	// Add masking key to header
	header = append(header, frame.MaskKey...)

	// Mask the payload
	maskedPayload := make([]byte, len(frame.Payload))
	for i := range frame.Payload {
		maskedPayload[i] = frame.Payload[i] ^ frame.MaskKey[i%4]
	}

	// Send frame
	if _, err := c.conn.Write(header); err != nil {
		return err
	}
	if _, err := c.conn.Write(maskedPayload); err != nil {
		return err
	}

	return nil
}

func (c *Client) SendTextMessage(message string) error {
	return c.sendFragmentedMessage([]byte(message), 0x1) // 0x1 for text frame
}

func (c *Client) SendBinaryMessage(data []byte) error {
	return c.sendFragmentedMessage(data, 0x2) // 0x2 for binary frame
}

func (c *Client) sendFragmentedMessage(data []byte, opcode byte) error {
	remaining := data
	firstFragment := true

	for len(remaining) > 0 {
		var chunk []byte
		isFinal := false

		if len(remaining) <= MaxFrameSize {
			chunk = remaining
			remaining = nil
			isFinal = true
		} else {
			chunk = remaining[:MaxFrameSize]
			remaining = remaining[MaxFrameSize:]
		}

		// Create frame for this fragment
		frame := &Frame{
			Fin:        isFinal,
			Opcode:     opcode,
			Masked:     true,
			PayloadLen: uint64(len(chunk)),
			MaskKey:    generateMaskKey(),
			Payload:    chunk,
		}

		// For continuation frames, use opcode 0x0
		if !firstFragment {
			frame.Opcode = 0x0 // Continuation frame
		}
		firstFragment = false

		if err := c.sendFrame(frame); err != nil {
			return fmt.Errorf("error sending frame fragment: %v", err)
		}
	}

	return nil
}

func (c *Client) ReadMessage() ([]byte, error) {
	message, err := c.ReadFullMessage()
	if err != nil {
		return nil, err
	}
	return message.Payload, nil
}

func (c *Client) ReadFullMessage() (*Message, error) {
	var fullMessage []byte
	var messageOpcode byte
	var message *Message

	for {
		frame, err := ReadFrame(c.conn)
		if err != nil {
			if err == io.EOF {
				return nil, fmt.Errorf("connection closed")
			}
			return nil, fmt.Errorf("error reading frame: %v", err)
		}

		switch frame.OpcodeName() {
		case "close":
			return nil, fmt.Errorf("received close frame")
		case "ping":
			c.sendPong(frame.Payload)
			continue
		case "pong":
			continue
		}

		// Handle fragmented messages
		if len(fullMessage) == 0 {
			// This is the first fragment
			messageOpcode = frame.Opcode
		} else if frame.Opcode != 0x0 {
			// Unexpected non-continuation frame
			return nil, fmt.Errorf("protocol error: expected continuation frame")
		}

		fullMessage = append(fullMessage, frame.Payload...)

		if frame.Fin {
			// Message is complete
			message = &Message{
				Type:    messageOpcode,
				Payload: fullMessage,
			}
			return message, nil
		}
	}
}

func (c *Client) sendPong(payload []byte) error {
	frame := &Frame{
		Fin:        true,
		Opcode:     0xA, // Pong frame
		Masked:     true,
		PayloadLen: uint64(len(payload)),
		MaskKey:    generateMaskKey(),
		Payload:    payload,
	}
	return c.sendFrame(frame)
}

// generateMaskKey generates a random 4-byte mask key using crypto/rand
func generateMaskKey() []byte {
	key := make([]byte, 4)
	if _, err := rand.Read(key); err != nil {
		// In case of error, return a fallback key
		// This is not ideal but better than panicking
		log.Printf("Error generating mask key: %v", err)
		return []byte{0x00, 0x00, 0x00, 0x00}
	}
	return key
}
