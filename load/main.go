package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/despondency/notifications-service/internal/notification"
	"github.com/google/uuid"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

const (
	numberOfNotifications = 10000
)

func main() {
	maxParallelism := make(chan struct{}, 100)
	wg := sync.WaitGroup{}

	n := make([][]byte, numberOfNotifications)
	for i := 0; i < numberOfNotifications; i++ {
		n[i] = createNotification(i)
	}
	t := time.Now()
	for i := 0; i < numberOfNotifications; i++ {
		v := i
		maxParallelism <- struct{}{}
		wg.Add(1)
		go func(idx int) {
			defer func() {
				<-maxParallelism
				wg.Done()
			}()
			sendNotification(n[idx])
		}(v)
	}
	wg.Wait()
	fmt.Printf(time.Since(t).String())
}

func createNotification(i int) []byte {
	n := &notification.Notification{
		UUID:            uuid.New().String(),
		NotificationTxt: fmt.Sprintf("txt-%d", i),
		Destination:     "EMAIL",
	}
	b, err := json.Marshal(n)
	if err != nil {
		panic(err)
	}
	return b
}

func sendNotification(buffer []byte) {
	n := rand.Intn(2)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, fmt.Sprintf("http://localhost:809%d/notification", n), bytes.NewBuffer(buffer))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		panic(err)
	}
	defer func() {
		resp.Body.Close()
	}()
}
