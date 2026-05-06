package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
)

var (
	data  map[string][]Record
	rwMux sync.RWMutex
)

func getFromCache(rc string) []Record {
	rwMux.RLock()
	defer rwMux.RUnlock()

	if records, ok := data[rc]; ok {
		return records
	}

	return nil
}

func addToCache(rc string, records []Record) {
	rwMux.Lock()
	defer rwMux.Unlock()
	if data == nil {
		data = make(map[string][]Record)
	}
	data[rc] = records
}

func GetRecords(rc string) ([]Record, error) {
	if rc == "" {
		return nil, fmt.Errorf("no request context provided")
	}

	if records := getFromCache(rc); records != nil {
		return records, nil
	}

	getUri := systemHost + "/get?rc=" + rc

	resp, err := http.Get(getUri)
	if err != nil {
		panic(err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Coudn't find request %s", rc)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	records := []Record{}

	if err = json.Unmarshal(data, &records); err != nil {
		return nil, err
	}

	addToCache(rc, records)

	return records, nil
}
