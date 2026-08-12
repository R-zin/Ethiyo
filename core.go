package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
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

func ge_bus_route(BusCode string) (string, string) {
	sp_url := "https://chalo.com/app/api/vasudha/track/"
	var sudha_sub string
	
	re := regexp.MustCompile(`route-live-info/([^/]+)/([^?]+)`)
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath("/Applications/Brave Browser.app/Contents/MacOS/Brave Browser"),
		chromedp.Flag("headless", true),
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
				match := re.FindStringSubmatch(ev.Request.URL)
				if len(match) > 0 {
					sudha_sub = match[0]
					cancel()
				}
			}
		}
	})
	var cookies []*network.Cookie
	err := chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("https://chalo.com/app/public-route/%s", BusCode)),
		chromedp.WaitVisible("body"),
		chromedp.ActionFunc(func(ctx context.Context) error {
			var err error
			cookies, err = network.GetCookies().Do(ctx)
			return err
		}),
	)
	if err != nil && !errors.Is(err, context.Canceled) {
		log.Fatal(err)
	}
	for _,cookie := range cookies {
		fmt.Println("Cookie Name:", cookie.Name)
		fmt.Println("Cookie Value:", cookie.Value)
	}
	// Extract cookies from browser context
	
	
	return sp_url + sudha_sub, ""
}

func makeReq(track_url string, cookie string) {
	client := &http.Client{}
	res, err := http.NewRequest("GET", track_url, nil)
	res.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/26.5.2 Safari/605.1.15")
	if cookie != "" {
		res.Header.Set("Cookie", cookie)
	}
	resp, err := client.Do(res)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	fmt.Println(resp.Status)
	fmt.Println(resp.Header)
	fmt.Println(resp.Body)

}
