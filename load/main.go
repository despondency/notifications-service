package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/despondency/notifications-service/internal/notification"
	"github.com/google/uuid"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

const (
	numberOfNotifications   = 50_000
	numberOfPushingRoutines = 500
)

var client = &http.Client{Transport: &http.Transport{
	TLSClientConfig:     &tls.Config{},
	MaxIdleConnsPerHost: 250,
}, Timeout: 60 * time.Second}

func main() {
	maxParallelism := make(chan struct{}, numberOfPushingRoutines)
	wg := sync.WaitGroup{}

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
			sendNotification(idx)
		}(v)
	}
	wg.Wait()
	fmt.Printf(time.Since(t).String())
}

func sendNotification(idx int) {
	n := &notification.Request{
		UUID:            uuid.New().String(),
		NotificationTxt: fmt.Sprintf("txt-%d", idx),
		Destination:     "EMAIL",
	}
	b, err := json.Marshal(n)
	if err != nil {
		panic(err)
	}
	rnd := rand.Intn(2)
	req, err := http.NewRequest(http.MethodPost, fmt.Sprintf("http://localhost:809%d/notification", rnd), bytes.NewBuffer(b))
	if err != nil {
		panic(err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	resp.Body.Close()
	req.Close = true
	_, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println("read body error", err.Error())
		panic(err)
	}
	//fmt.Printf("response: %s\n", content)
}
