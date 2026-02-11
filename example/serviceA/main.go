package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"example.com/sdk"
)

func main() {
	sdk.Init("ServiceA", "http://localhost:8080", true, true)

	sdk.HandleFunc("/boost", getHandler)

	if err := http.ListenAndServe(":3000", nil); err != nil {
		panic(err)
	}
}

type BoostFactor struct {
	Name        string `json:"n"`
	BoostFactor int    `json:"bf"`
}

type Boost struct {
	Name  string  `json:"n"`
	Boost float64 `json:"b"`
}

func getHandler(w http.ResponseWriter, r *http.Request) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("Recovered in f", r)
			w.WriteHeader(http.StatusInternalServerError)
		}
	}()

	name := r.URL.Query().Get("name")

	const serviceBUrl = "http://localhost:3001/boost-factor?name="

	data, err := sdk.GetWithContext(r.Context(), serviceBUrl+name)
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

	boostFactor := BoostFactor{}
	err = json.Unmarshal(body, &boostFactor)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	resp := Boost{
		Name:  boostFactor.Name,
		Boost: 1.0 / float64(boostFactor.BoostFactor),
	}

	bytes, err := json.Marshal(resp)
	if err != nil {
		fmt.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Write(bytes)
}
