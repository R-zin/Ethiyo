package main

import (
	"fmt"
	"log"
	"net/http"
)

func request(BusCode string) {
	url := fmt.Sprintf("https://chalo.com/app/public-route/%s", BusCode)
	resp, err := http.Get(url)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.Body)
}
