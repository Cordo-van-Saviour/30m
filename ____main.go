package main

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/bits-and-blooms/bitset"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

const (
	numCheckboxes = 1_000_000
	batchSize     = 1
	redisKey      = "checkboxes"
)

var (
	upgrader = websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	rdb           *redis.Client
	checkboxes    *bitset.BitSet
	checkboxesMux sync.RWMutex
	ctx           = context.Background()
	updateChan    = make(chan updateMessage, batchSize)
)

type updateMessage struct {
	Index int  `json:"index"`
	Value bool `json:"value"`
}

func main() {
	// Initialize Redis client
	rdb = redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	// Initialize bitset
	checkboxes = bitset.New(numCheckboxes)

	// Load initial state from Redis
	loadFromRedis()

	// Start update handler
	go handleUpdates()

	// Set up HTTP routes
	http.HandleFunc("/", serveHome)
	http.HandleFunc("/ws", handleWebSocket)

	// Start server
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func serveHome(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.Error(w, "Not found", http.StatusNotFound)
		return
	}
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	checkboxesMux.RLock()
	defer checkboxesMux.RUnlock()

	// Convert bitset to a slice of booleans
	data, err := rdb.Get(ctx, redisKey).Bytes()
	if err != nil {
		if err == redis.Nil {
			// If no data found in Redis, return an empty state
			data = make([]byte, numCheckboxes)
		} else {
			log.Printf("Error loading from Redis: %v", err)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}
	}

	w.Write(data)
}

func handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}
	defer conn.Close()

	for {
		var msg updateMessage
		err := conn.ReadJSON(&msg)
		if err != nil {
			log.Println(err)
			break
		}
		updateChan <- msg
	}
}

func handleUpdates() {
	// add some logging here
	log.Println("Starting update handler")

	batch := make([]updateMessage, 0, batchSize)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case msg := <-updateChan:
			log.Printf("Received update: %v", msg)
			batch = append(batch, msg)
			if len(batch) >= batchSize {
				processBatch(batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				processBatch(batch)
				batch = batch[:0]
			}
		}
	}
}

func processBatch(batch []updateMessage) {
	checkboxesMux.Lock()
	defer checkboxesMux.Unlock()

	for _, msg := range batch {
		if msg.Value {
			checkboxes.Set(uint(msg.Index))
		} else {
			checkboxes.Clear(uint(msg.Index))
		}
	}

	saveToRedis()
}

func loadFromRedis() {
	checkboxesMux.Lock()
	defer checkboxesMux.Unlock()

	log.Println("Loading state from Redis")
	data, err := rdb.Get(ctx, redisKey).Bytes()
	if err != nil && err != redis.Nil {
		log.Printf("Error loading from Redis: %v", err)
		return
	}

	if err == redis.Nil {
		log.Println("No data found in Redis, starting with empty state")
		return
	}

	err = checkboxes.UnmarshalBinary(data)
	if err != nil {
		log.Printf("Error unmarshaling bitset: %v", err)
	}
}

func saveToRedis() {
	data, err := checkboxes.MarshalBinary()
	if err != nil {
		log.Printf("Error marshaling bitset: %v", err)
		return
	}

	err = rdb.Set(ctx, redisKey, data, 0).Err()
	if err != nil {
		log.Printf("Error saving to Redis: %v", err)
	}
}
