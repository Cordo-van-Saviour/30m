package main

import (
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/bits-and-blooms/bitset"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const TotalBits = 1000000

type Server struct {
	bitset      *bitset.BitSet
	redisClient *redis.Client
	clients     map[*websocket.Conn]chan<- string
	mu          sync.RWMutex
	updates     chan Update
}

type Update struct {
	Index uint
	Value bool
}

func NewServer() *Server {
	redisAddr := os.Getenv("REDIS_URL")
	if redisAddr == "" {
		redisAddr = "redis:6379" // Default to the service name if env var is not set
	}

	redisClient := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})

	return &Server{
		bitset:      bitset.New(TotalBits),
		redisClient: redisClient,
		clients:     make(map[*websocket.Conn]chan<- string),
		updates:     make(chan Update, 100),
	}
}

func (s *Server) LoadFromRedis() error {
	data, err := s.redisClient.Get(context.Background(), "bitset").Bytes()
	if err != nil && err != redis.Nil {
		return err
	}
	if err == redis.Nil {
		return s.SaveToRedis()
	}
	s.bitset.UnmarshalBinary(data)
	return nil
}

func (s *Server) SaveToRedis() error {
	data, _ := s.bitset.MarshalBinary()
	return s.redisClient.Set(context.Background(), "bitset", data, 0).Err()
}

func (s *Server) UpdateBit(index uint, value bool) error {
	if index >= TotalBits {
		return fmt.Errorf("index out of range: %d", index)
	}

	s.mu.Lock()
	s.bitset.SetTo(index, value)
	s.mu.Unlock()

	err := s.SaveToRedis()
	if err != nil {
		return err
	}

	s.updates <- Update{Index: index, Value: value}
	return nil
}

func (s *Server) BroadcastUpdates() {
	for update := range s.updates {
		message := fmt.Sprintf("%d:%t", update.Index, update.Value)
		s.mu.RLock()
		for _, ch := range s.clients {
			select {
			case ch <- message:
			default:
				// If the channel is full, skip this client
			}
		}
		s.mu.RUnlock()
	}
}

func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	messageChan := make(chan string, 10)
	s.mu.Lock()
	s.clients[conn] = messageChan
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.clients, conn)
		close(messageChan)
		s.mu.Unlock()
	}()

	go func() {
		for message := range messageChan {
			err := conn.WriteMessage(websocket.TextMessage, []byte(message))
			if err != nil {
				log.Println("Error writing to WebSocket:", err)
				return
			}
		}
	}()

	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}

		parts := strings.Split(string(message), ":")
		if len(parts) != 2 {
			log.Println("Invalid message format")
			continue
		}

		index, err := strconv.ParseUint(parts[0], 10, 32)
		if err != nil {
			log.Println("Invalid index:", err)
			continue
		}

		value, err := strconv.ParseBool(parts[1])
		if err != nil {
			log.Println("Invalid value:", err)
			continue
		}

		fmt.Printf("Received update: %d:%t\n", index, value)
		err = s.UpdateBit(uint(index), value)
		if err != nil {
			log.Println("Failed to update bit:", err)
			continue
		}
	}
}

func main() {
	server := NewServer()
	err := server.LoadFromRedis()
	if err != nil {
		log.Fatal("Failed to load data from Redis:", err)
	}

	go server.BroadcastUpdates()

	http.HandleFunc("/ws", server.HandleWebSocket)
	http.HandleFunc("/api", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		err = server.LoadFromRedis()
		if err != nil {
			log.Println("Failed to load data from Redis:", err)
			return
		}

		server.mu.RLock()
		bytes := server.bitset.Bytes()
		server.mu.RUnlock()

		byteSlice := make([]byte, len(bytes)*8)
		for i, word := range bytes {
			binary.LittleEndian.PutUint64(byteSlice[i*8:], word)
		}
		encoded := base64.StdEncoding.EncodeToString(byteSlice)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"bitset": encoded})
	})

	log.Println("Server starting on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

