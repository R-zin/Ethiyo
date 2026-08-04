package main

import (
	"fmt"
	"log"
	"net/http"
	"github.com/gocolly/colly"
)

func scrap(url string) {
	c := colly.NewCollector()
	c.OnHTML("body", func(e *colly.HTMLElement) {})
}


func request(BusCode string) {

	url := fmt.Sprintf("https://chalo.com/app/public-route/%s", BusCode)
	req, err := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.5.2 Safari/605.1.15")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(resp.Body.Read())
	defer resp.Body.Close()

func ge_bus_route(Bus_url string) {


	}
}
