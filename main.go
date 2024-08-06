package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

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
	clients     map[*websocket.Conn]bool
}

func NewServer() *Server {
	// Initialize Redis client
	redisClient := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	return &Server{
		bitset:      bitset.New(TotalBits),
		redisClient: redisClient,
		clients:     make(map[*websocket.Conn]bool),
	}
}

func (s *Server) LoadFromRedis() error {
	// Load data from Redis into bitset
	data, err := s.redisClient.Get(context.Background(), "bitset").Bytes()
	if err != nil && err != redis.Nil {
		return err
	}
	if err == redis.Nil {
		// Initialize empty bitset in Redis
		return s.SaveToRedis()
	}
	s.bitset.UnmarshalBinary(data)
	return nil
}

func (s *Server) SaveToRedis() error {
	// Save bitset to Redis
	data, _ := s.bitset.MarshalBinary()
	return s.redisClient.Set(context.Background(), "bitset", data, 0).Err()
}

func (s *Server) UpdateBit(index uint, value bool) error {
	if index >= TotalBits {
		return fmt.Errorf("index out of range: %d", index)
	}

	s.bitset.SetTo(index, value)
	err := s.SaveToRedis()
	if err != nil {
		return err
	}
	s.BroadcastUpdate(index, value)
	return nil
}

func (s *Server) BroadcastUpdate(index uint, value bool) {
	message := fmt.Sprintf("%d:%t", index, value)
	for client := range s.clients {
		client.WriteMessage(websocket.TextMessage, []byte(message))
	}
}

// WebSocket handler
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	s.clients[conn] = true

	// Send initial state
	bools := make([]bool, TotalBits)
	for i := uint(0); i < TotalBits; i++ {
		bools[i] = s.bitset.Test(i)
	}
	jsonData, _ := json.Marshal(bools)
	conn.WriteMessage(websocket.TextMessage, jsonData)

	// Handle incoming messages
	for {
		_, message, err := conn.ReadMessage()
		if err != nil {
			break
		}
		// Parse message and update bit
		// Format: "index:value"
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

	http.HandleFunc("/ws", server.HandleWebSocket)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// allow CORS
		w.Header().Set("Access-Control-Allow-Origin", "*")
		// serve data here
		err = server.LoadFromRedis()
		if err != nil {
			log.Println("Failed to load data from Redis:", err)
			return
		}

		// turn data into an array of 0s and 1s
		// this is a bitset, so we need to know the total number of bits
		var bools []bool
		for i := uint(0); i < TotalBits; i++ {
			bools = append(bools, server.bitset.Test(i))
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(bools)
	})

	log.Println("Server starting on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}
