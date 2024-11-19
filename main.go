package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"log"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"30m/rle"

	"github.com/bits-and-blooms/bitset"
	"github.com/chai2010/webp"
	"github.com/gorilla/websocket"
	"github.com/redis/go-redis/v9"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

const TotalBits = 1_000_000_000

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
		redisAddr = "localhost:6379" // Default to the service name if env var is not set
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

	http.HandleFunc("/image", server.HandleImageAPI)
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

		// encoded1 := encodeToBase64(bytes)
		rleUint64 := rleEncodeToUint64(bytes)
		encoded := base64.StdEncoding.EncodeToString(rleUint64)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{
			// "base64":    encoded1,
			"bitsetRLE": encoded,
		})
	})

	log.Println("Server starting on :8080")
	err = http.ListenAndServe(":8080", nil)
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}
}

func rleEncodeToUint64(bytes []uint64) []byte {
	uint64Slice := make([]uint64, len(bytes))
	for i, word := range bytes {
		uint64Slice[i] = word
	}
	rleEncoded := rle.EncodeUint64(uint64Slice)
	return rleEncoded
}

func rleDecodeFromUint64(rleEncoded []byte) []uint64 {
	rleDecoded, err := rle.DecodeUint64(rleEncoded)
	if err != nil {
		log.Fatal("Error occured while decoding")
	}

	return rleDecoded
}

func encodeToBase64(bytes []uint64) string {
	byteSlice := make([]byte, len(bytes)*8)
	for i, word := range bytes {
		binary.LittleEndian.PutUint64(byteSlice[i*8:], word)
	}
	encoded := base64.StdEncoding.EncodeToString(byteSlice)
	return encoded
}

func (s *Server) HandleImageAPI(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	webpData, err := EncodeToWebP(s.bitset)
	s.mu.RUnlock()

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "image/webp")
	w.Write(webpData)
}

func EncodeToWebP(bs *bitset.BitSet) ([]byte, error) {
	totalBits := int(bs.Len())
	bitsPerPixel := 24
	totalPixels := (totalBits + bitsPerPixel - 1) / bitsPerPixel
	width := int(math.Sqrt(float64(totalPixels)))
	height := (totalPixels + width - 1) / width

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	pixelIndex := 0
	for bitIndex := uint(0); bitIndex < bs.Len(); bitIndex += 24 {
		if pixelIndex >= width*height {
			break
		}

		r := uint8(0)
		g := uint8(0)
		b := uint8(0)

		// Red channel
		for i := uint(0); i < 8 && (bitIndex+i) < bs.Len(); i++ {
			if bs.Test(bitIndex + i) {
				r |= 1 << i
			}
		}

		// Green channel
		for i := uint(0); i < 8 && (bitIndex+8+i) < bs.Len(); i++ {
			if bs.Test(bitIndex + 8 + i) {
				g |= 1 << i
			}
		}

		// Blue channel
		for i := uint(0); i < 8 && (bitIndex+16+i) < bs.Len(); i++ {
			if bs.Test(bitIndex + 16 + i) {
				b |= 1 << i
			}
		}

		x := pixelIndex % width
		y := pixelIndex / width
		img.Set(x, y, color.RGBA{r, g, b, 255})
		pixelIndex++
	}

	var buf bytes.Buffer
	options := &webp.Options{
		Lossless: true,
		Quality:  100,
	}
	if err := webp.Encode(&buf, img, options); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func DecodeFromWebP(data []byte) (*bitset.BitSet, error) {
	img, err := webp.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	bounds := img.Bounds()
	width := bounds.Max.X
	height := bounds.Max.Y
	bs := bitset.New(uint(width * height * 24))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			pixelIndex := y*width + x
			r, g, b, _ := img.At(x, y).RGBA()

			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)

			baseIndex := uint(pixelIndex * 24)

			// Red channel
			for i := uint(0); i < 8; i++ {
				if r8&(1<<i) != 0 {
					bs.Set(baseIndex + i)
				}
			}

			// Green channel
			for i := uint(0); i < 8; i++ {
				if g8&(1<<i) != 0 {
					bs.Set(baseIndex + 8 + i)
				}
			}

			// Blue channel
			for i := uint(0); i < 8; i++ {
				if b8&(1<<i) != 0 {
					bs.Set(baseIndex + 16 + i)
				}
			}
		}
	}

	return bs, nil
}
