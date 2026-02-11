package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"example.com/sdk"
)

func main() {
	sdk.Init("ServiceC", "http://localhost:8080", true, true)

	sdk.HandleFunc("/hit-count", getHandler)

	if err := http.ListenAndServe(":3002", nil); err != nil {
		panic(err)
	}
}

var counter = map[string]int{}

type HitCount struct {
	Name     string `json:"n"`
	HitCount int    `json:"hc"`
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	counter[name]++

	res := HitCount{
		Name:     strings.ToUpper(name),
		HitCount: counter[name],
	}

	bytes, err := json.Marshal(res)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(bytes)
}
