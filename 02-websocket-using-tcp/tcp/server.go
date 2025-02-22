package tcp

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
)

type Msg struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

/**
 * WebSocket Frame.
 */
type Frame struct {
	Fin        bool   // Fin indicates if this is the final fragment in a message.
	Opcode     byte   // Opcode defines the interpretation of the payload data.
	Masked     bool   // Masked indicates if the payload data is masked.
	PayloadLen uint64 // PayloadLen specifies the length of the payload data.
	MaskKey    []byte // MaskKey is the masking key used to unmask the payload data.
	Payload    []byte // Payload contains the actual data being transmitted.
}

/**
 * * ReadFrame reads a single WebSocket frame from a TCP connection.
 *
 * * In the WebSocket protocol, the first byte of a frame contains several important pieces of information. Let's break down the first byte:
 * FIN bit (1 bit): The Most Significant Bit (MSB) of the first byte (bit 7) indicates whether this is the final fragment in a message. If set to 1, it means this is the final fragment.
 * RSV1, RSV2, RSV3 bits (3 bits): The next three bits (bits 6, 5, and 4) are reserved for future use. They should be set to 0 unless an extension defines otherwise. These bits are typically not used in standard WebSocket communication.
 * Opcode (4 bits): The last four bits (bits 3 to 0) of the first byte define the frame's type. For example:
 * 	 	0x0 (0000): Continuation frame
 * 		0x1 (0001): Text frame
 * 		0x2 (0010): Binary frame
 * 		0x8 (1000): Connection close frame
 * 		0x9 (1001): Ping frame
 * 		0xA (1010): Pong frame
 */
func ReadFrame(conn net.Conn) (*Frame, error) {
	frame := &Frame{}

	firstByte := make([]byte, 1)
	if _, err := io.ReadFull(conn, firstByte); err != nil {
		return nil, err
	}

	/**
	 * Note: 1 Byte is 8 bits.
	 * Anything bitwise ( & ) with above will be either 0x80 or 0
	 * 0x80 -> 1000 0000
	 * 0x0F -> 0000 1111
	 *
	 * 	frame.Fin extracts the FIN bit to determine if this is the final fragment.
	 * 	frame.Opcode extracts the last four bits to determine the frame type.
	 * 	The reserved bits (RSV1, RSV2, RSV3) are not used in this code, which is typical unless you are implementing or using WebSocket extensions that require these bits.
	 */
	frame.Fin = (firstByte[0] & 0x80) != 0 // Determines whether the MSB is 1.
	frame.Opcode = firstByte[0] & 0x0F     // Determines the right 4 bits from first byte.

	log.Printf("Fin: %v \n", frame.Fin)
	secondByte := make([]byte, 1)
	if _, err := io.ReadFull(conn, secondByte); err != nil {
		return nil, err
	}

	/**
	 * Note: 1 Byte is 8 bits.
	 * Anything bitwise ( & ) with above will be either 0x80 or 0
	 * 0x80 -> 1000 0000
	 * 0x7F -> 0111 1111
	 *
	 * 	frame.Masked extracts the MASK bit to determine if the payload data is masked.
	 * 	payloadLen extracts the last seven bits to determine the payload length.
	 */
	frame.Masked = (secondByte[0] & 0x80) != 0 // Determine the whether the MSB is 1
	payloadLen := secondByte[0] & 0x7F

	// Read extended payload length if necessary
	switch payloadLen {
	case 126: // 0111 1110 -> 0x7E
		extendedLen := make([]byte, 2)
		if _, err := io.ReadFull(conn, extendedLen); err != nil {
			return nil, err
		}
		frame.PayloadLen = uint64(binary.BigEndian.Uint16(extendedLen))
	case 127: // 0111 1111 -> 0x7F
		extendedLen := make([]byte, 8)
		if _, err := io.ReadFull(conn, extendedLen); err != nil {
			return nil, err
		}
		frame.PayloadLen = binary.BigEndian.Uint64(extendedLen)
	default:
		frame.PayloadLen = uint64(payloadLen)
	}

	/**
	 * Note: 1 Byte is 8 bits.
	 * Anything bitwise ( & ) with above will be either 0x80 or 0
	 *
	 * 	frame.MaskKey extracts the MASK KEY bit.
	 *  MASK KEY is of 4 byte.
	 *
	 */
	if frame.Masked {
		frame.MaskKey = make([]byte, 4)
		if _, err := io.ReadFull(conn, frame.MaskKey); err != nil {
			return nil, err
		}
	}

	// Read payload
	if frame.PayloadLen > 0 {
		frame.Payload = make([]byte, frame.PayloadLen)
		if _, err := io.ReadFull(conn, frame.Payload); err != nil {
			return nil, err
		}

		/**
		 * Certainly! Let's break down the operation in the provided Go code:
		 *
		 * 	Explanation
		 * 		frame.Payload[i]:
		 *
		 * 	This accesses the i-th element of the Payload slice (or array)
		 * 	within the frame struct. The Payload is likely a byte slice ([]byte).
		 * 		frame.MaskKey[i%4]:
		 *
		 * 	This accesses an element of the MaskKey slice (or array) within the frame struct.
		 * 	The index used here is i%4, which means the index is the remainder of i divided by 4.
		 * 	This ensures that the index cycles through 0, 1, 2, and 3, regardless of how large i gets.
		 * 		^= (XOR assignment operator):
		 *
		 * 	The ^= operator performs a bitwise XOR operation between the left-hand side and
		 * 	the right-hand side, and then assigns the result back to the left-hand side.
		 * 	In this case, it XORs frame.Payload[i] with frame.MaskKey[i%4] and stores the
		 * 	result back in frame.Payload[i].
		 *
		 * 	Context
		 * 		This operation is commonly used in WebSocket implementations for masking and
		 * 		unmasking data frames. The WebSocket protocol specifies that payload data must
		 * 		be XORed with a masking key to obscure the data being transmitted.
		 *
		 * 	Example
		 * 		Let's say frame.Payload is [0x01, 0x02, 0x03, 0x04] and
		 * 		frame.MaskKey is [0xAA, 0xBB, 0xCC, 0xDD]. For i = 0, the operation would be:
		 *
		 * 	After the operation, frame.Payload would be [0xAB, 0x02, 0x03, 0x04].
		 *
		 * 	This process would repeat for each element in frame.Payload, cycling through the MaskKey.
		 *
		 * 	Summary
		 * 		The line of code is performing a bitwise XOR operation between each byte of the Payload and
		 * 		a corresponding byte from the MaskKey, cycling through the MaskKey every 4 bytes.
		 * 		This is typically used for encoding or decoding data in WebSocket frames.
		 */
		if frame.Masked {
			for i := range frame.Payload {
				frame.Payload[i] ^= frame.MaskKey[i%4]
			}
		}
	}

	return frame, nil
}

// OpcodeName returns the string representation of the opcode
func (f *Frame) OpcodeName() string {
	switch f.Opcode {
	case 0x0:
		return "continuation"
	case 0x1:
		return "text"
	case 0x2:
		return "binary"
	case 0x8:
		return "close"
	case 0x9:
		return "ping"
	case 0xA:
		return "pong"
	default:
		return "unknown"
	}
}

func NewServer(wg *sync.WaitGroup) {
	defer wg.Done()

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		log.Println("Error starting WebSocket server:", err)
		return
	}
	defer listener.Close()

	log.Printf("WebSocket Server running on port %d\n", port)

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Println("Error accepting WebSocket connection:", err)
			continue
		}
		go handleWebSocket(conn)
	}
}

func handleWebSocket(conn net.Conn) {
	defer conn.Close()

	// Step 1: Perform WebSocket handshake
	reader := bufio.NewReader(conn)
	request, err := http.ReadRequest(reader)
	if err != nil {
		log.Println("Error reading HTTP request:", err)
		return
	}

	// Validate WebSocket handshake
	if !strings.Contains(request.Header.Get("Connection"), "Upgrade") ||
		request.Header.Get("Upgrade") != "websocket" {
		log.Println("Invalid WebSocket handshake")
		return
	}

	// WebSocket handshake response
	key := request.Header.Get("Sec-WebSocket-Key")
	acceptKey := generateWebSocketAcceptKey(key)
	response := fmt.Sprintf(
		"HTTP/1.1 101 Switching Protocols\r\n"+
			"Upgrade: websocket\r\n"+
			"Connection: Upgrade\r\n"+
			"Sec-WebSocket-Accept: %s\r\n\r\n",
		acceptKey,
	)
	_, err = conn.Write([]byte(response))
	if err != nil {
		log.Println("Error sending handshake response:", err)
		return
	}
	log.Println("WebSocket handshake completed")

	// Step 2: Handle WebSocket frames
	for {
		frame, err := ReadFrame(conn)
		if err != nil {
			if err == io.EOF {
				log.Println("Client disconnected")
			} else {
				log.Println("Error reading WebSocket frame:", err)
			}
			return
		}

		log.Printf("Received frame type: %s", frame.OpcodeName())
		if len(frame.Payload) > 0 {
			log.Printf("Payload: %s", string(frame.Payload))
		}

		// Handle different frame types
		switch frame.OpcodeName() {
		case "close":
			log.Println("Closing connection")
			return
		case "ping":
			log.Println("Received ping")
		case "pong":
			log.Println("Received pong")
		case "text":
			var msg Msg
			err := json.Unmarshal(frame.Payload, &msg)
			if err != nil {
				log.Println("Error parsing JSON:", err)
				continue
			}
			log.Printf("Received message: %s", msg.Content)

			response := Msg{Role: "agent", Content: "Okay i got it"}
			responseJSON, _ := json.Marshal(response)
			sendFrame(conn, responseJSON)
		}
	}
}

func sendFrame(conn net.Conn, payload []byte) {
	frame := &Frame{
		Fin:        true,
		Opcode:     0x1, // Text frame
		PayloadLen: uint64(len(payload)),
		Payload:    payload,
	}

	header := []byte{0x81} // FIN + Text frame opcode
	if frame.PayloadLen <= 125 {
		header = append(header, byte(frame.PayloadLen))
	} else if frame.PayloadLen <= 65535 {
		header = append(header, 126)
		extendedLen := make([]byte, 2)
		binary.BigEndian.PutUint16(extendedLen, uint16(frame.PayloadLen))
		header = append(header, extendedLen...)
	} else {
		header = append(header, 127)
		extendedLen := make([]byte, 8)
		binary.BigEndian.PutUint64(extendedLen, frame.PayloadLen)
		header = append(header, extendedLen...)
	}

	conn.Write(header)
	conn.Write(payload)
}

func generateWebSocketAcceptKey(key string) string {
	h := sha1.New()
	h.Write([]byte(key + "258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

const port = 4443
