package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"example.com/sdk"
)

func main() {
	sdk.Init("ServiceB", "http://localhost:8080", true, true)

	sdk.HandleFunc("/boost-factor", getHandler)

	if err := http.ListenAndServe(":3001", nil); err != nil {
		panic(err)
	}
}

type HitCount struct {
	Name     string `json:"n"`
	HitCount int    `json:"hc"`
}

type BoostFactor struct {
	Name        string `json:"n"`
	BoostFactor int    `json:"bf"`
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")

	const serviceCUrl = "http://localhost:3002/hit-count?name="

	data, err := sdk.GetWithContext(r.Context(), serviceCUrl+name)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	body, err := io.ReadAll(data.Body)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	hitCount := HitCount{}
	err = json.Unmarshal(body, &hitCount)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := BoostFactor{
		Name:        hitCount.Name,
		BoostFactor: (hitCount.HitCount % 17), // Max boost 16 and then roll over
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(bytes)
}
