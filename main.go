package main

import (
	"container/heap"
	"fmt"

	// "encoding/json"

	"log"
	"net"
	"time"

	"github.com/gorilla/websocket"
)

type StockData struct {
	StockName string
	Data      string
	Timestamp time.Time
}

type PriorityQueue []*StockData

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].Timestamp.After(pq[j].Timestamp)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*StockData)
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

type WebSocketConnection struct {
	conn             *websocket.Conn
	subscribedStocks map[string]bool
}

var clients map[*WebSocketConnection]bool

func handleConnection(conn net.PacketConn, c chan<- *StockData) {
	buffer := make([]byte, 1024)
	n, _, err := conn.ReadFrom(buffer)
	if err != nil {
		if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
			return // If there's a timeout, just return and end this goroutine
		} else {
			log.Fatal(err) // If it's another error, log it and exit
		}
	}

	var stockData StockData
	fmt.Printf("reading stock data : %d\n", buffer[:n])
	// err = json.Unmarshal(buffer[:n], &stockData)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	c <- &stockData
}

func startServer(c chan<- *StockData, maxWorkers int) {
	ln, err := net.ListenPacket("udp", "192.168.1.66:7701")
	if err != nil {
		log.Fatal(err)
	}
	defer ln.Close()

	// Create a worker pool
	workers := make(chan struct{}, maxWorkers)

	for {
		workers <- struct{}{}
		go func() {
			handleConnection(ln, c)
			<-workers
		}()
		time.Sleep(1 * time.Millisecond) // Add a small delay to prevent creating too many goroutines
	}
}

func main() {
	// Define a buffered channel to communicate between goroutines
	dataChannel := make(chan *StockData, 1000)

	// Define your ports
	// ports := []string{"7001"}

	maxWorkers := 100
	// for _, port := range ports {
	// 	go startServer(port, dataChannel, maxWorkers)
	// }
	go startServer(dataChannel, maxWorkers)

	// Start a goroutine to handle data and send it to frontend
	go func() {
		pq := &PriorityQueue{}
		heap.Init(pq)
		for data := range dataChannel {
			heap.Push(pq, data)
			// Handle data and send it to frontend
			for client := range clients {
				if client.subscribedStocks[data.StockName] {
					err := client.conn.WriteJSON(data)
					if err != nil {
						log.Fatal(err)
					}
				}
			}
		}
	}()

	// Prevent main from exiting
	select {}
}
