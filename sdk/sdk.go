package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

var (
	serviceName  string
	logEnabled   bool
	debugEnabled bool
	systemHost   string
	debugHost    string
	feeder       chan<- Record
	cancelFunc   context.CancelFunc
)

func Init(name, host string, log, debug bool) {
	serviceName = name
	systemHost = host
	debugHost = systemHost + "/runtime/proxy"
	logEnabled = log
	debugEnabled = debug
	InstrumentClient(DefaultClient)
	InstrumentClient(http.DefaultClient) // This is to make force an error when http.Get or http.Post is called
	feeder, cancelFunc = processInBackground(systemHost)
}

func Close() {
	if cancelFunc != nil {
		cancelFunc()
	}
}

func Log(r Record) {
	if feeder != nil {
		feeder <- r
	}
}

func processInBackground(host string) (chan<- Record, context.CancelFunc) {
	feeder := make(chan Record)
	ctx, cancelFunc := context.WithCancel(context.Background())

	go func(feederChan <-chan Record, ctx context.Context) {
		records := make([]Record, 0, 100)
		postUrl := host + "/runtime/record"
		client := &http.Client{}

		for {
			select {
			case r := <-feederChan:
				records = append(records, r)
			case <-time.Tick(5 * time.Second):

				data := make([]Record, len(records))
				copy(data, records)
				records = records[:0]

				if len(data) > 0 {
					go func(data []Record) {
						body, err := json.Marshal(data)
						if err != nil {
							fmt.Printf("Unable to marshal payload: %s", err.Error())
							return
						}

						resp, err := client.Post(postUrl, "application/json", bytes.NewBuffer(body))
						if err != nil {
							fmt.Printf("Unable to post payload: %s", err.Error())
							return
						}
						if resp.StatusCode != http.StatusAccepted {
							fmt.Printf("Invalid status code received: %d\n", resp.StatusCode)
						}
					}(data)
				}
			case <-ctx.Done():
				return
			}
		}
	}(feeder, ctx)

	return feeder, cancelFunc
}
