package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
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
	defer resp.Body.Close()
}

func ge_bus_route(BusCode string) {
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath("/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"),
		chromedp.Flag("headless", true), // Change to true if you don't want the browser window
	)
	allocCtx, cancel := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()
	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		log.Fatal(err)
	}
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *network.EventRequestWillBeSent:
			if ev.Type == network.ResourceTypeXHR || ev.Type == network.ResourceTypeFetch {
				fmt.Println(ev.Request.URL)

			}
		}
	})
	err := chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("https://chalo.com/app/public-route/%s", BusCode)),
		chromedp.WaitVisible("body"))
	if err != nil {
		log.Fatal(err)
	}
}
