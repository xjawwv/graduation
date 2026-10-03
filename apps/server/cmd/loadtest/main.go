package main

import (
	"flag"
	"fmt"
	"log"
	"math/rand/v2"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/gorilla/websocket"
)

type counters struct {
	connected          atomic.Uint64
	connectionFailures atomic.Uint64
	sent               atomic.Uint64
	messageErrors      atomic.Uint64
}

func main() {
	wsURL := flag.String("url", "ws://localhost:8080/ws", "WebSocket endpoint")
	room := flag.String("room", "GRAD26", "room code")
	users := flag.Int("users", 100, "simulated students")
	rate := flag.Float64("rate", 2, "reactions per second per user")
	duration := flag.Duration("duration", 60*time.Second, "test duration")
	flag.Parse()
	if *users < 1 || *rate <= 0 || *duration <= 0 {
		flag.Usage()
		os.Exit(2)
	}
	parsed, err := url.Parse(*wsURL)
	if err != nil {
		log.Fatal(err)
	}
	stop := make(chan struct{})
	var once sync.Once
	stopAll := func() { once.Do(func() { close(stop) }) }
	timer := time.AfterFunc(*duration, stopAll)
	defer timer.Stop()
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case <-signals:
			stopAll()
		case <-stop:
		}
	}()
	var stats counters
	var wg sync.WaitGroup
	start := time.Now()
	for user := range *users {
		wg.Add(1)
		go runUser(user, parsed.String(), *room, *rate, stop, &stats, &wg)
	}
	wg.Wait()
	elapsed := time.Since(start)
	fmt.Printf("\nLoad test complete\nRoom: %s\nRequested users: %d\nConnected: %d\nConnection failures: %d\nReactions sent: %d\nMessage errors: %d\nDuration: %s\nSend throughput: %.1f msg/s\n", *room, *users, stats.connected.Load(), stats.connectionFailures.Load(), stats.sent.Load(), stats.messageErrors.Load(), elapsed.Round(time.Millisecond), float64(stats.sent.Load())/elapsed.Seconds())
}
func runUser(id int, endpoint, room string, rate float64, stop <-chan struct{}, stats *counters, wg *sync.WaitGroup) {
	defer wg.Done()
	conn, _, err := websocket.DefaultDialer.Dial(endpoint, nil)
	if err != nil {
		stats.connectionFailures.Add(1)
		return
	}
	defer conn.Close()
	join := map[string]any{"type": "join", "role": "student", "room": room, "client_id": fmt.Sprintf("load-%d-%d", time.Now().UnixNano(), id)}
	if err = conn.WriteJSON(join); err != nil {
		stats.connectionFailures.Add(1)
		return
	}
	stats.connected.Add(1)
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		for {
			if _, _, readErr := conn.ReadMessage(); readErr != nil {
				return
			}
		}
	}()
	reactions := []string{"❤️", "🔥", "😂", "😭", "👏", "🎓"}
	interval := time.Duration(float64(time.Second) / rate)
	jitter := time.Duration(rand.Int64N(max(int64(interval), 1)))
	first := time.NewTimer(jitter)
	select {
	case <-first.C:
	case <-stop:
		first.Stop()
		return
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-stop:
			_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, "done"), time.Now().Add(time.Second))
			return
		case <-readDone:
			stats.messageErrors.Add(1)
			return
		case <-ticker.C:
			if err = conn.WriteJSON(map[string]string{"type": "reaction", "emoji": reactions[rand.IntN(len(reactions))]}); err != nil {
				stats.messageErrors.Add(1)
				return
			}
			stats.sent.Add(1)
		}
	}
}
