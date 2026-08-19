package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/chromedp"
)

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

func get_path_code(BusCode string) string {
	var track_url string
	reTrack := regexp.MustCompile(
		`https://chalo\.com/app/api/vasudha/track/route-live-info/[^?]+`,
	)

	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		),
		chromedp.Flag("headless", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(
		context.Background(),
		opts...,
	)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		log.Printf("network enable error: %v", err)
		return ""
	}

	done := make(chan struct{})

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		req, ok := ev.(*network.EventRequestWillBeSent)
		if !ok {
			return
		}
		if req.Type != network.ResourceTypeXHR && req.Type != network.ResourceTypeFetch {
			return
		}
		if match := reTrack.FindString(req.Request.URL); match != "" {
			track_url = match
			select {
			case <-done:
			default:
				close(done)
			}
		}
	})

	if err := chromedp.Run(ctx,
		chromedp.Navigate(fmt.Sprintf("https://chalo.com/app/public-route/%s", BusCode)),
		chromedp.WaitVisible("body"),
	); err != nil {
		log.Printf("navigate error: %v", err)
		return ""
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		log.Printf("timed out waiting for track URL")
		return ""
	}

	return track_url

}

func getBusRoute(BusCode string) (string, string, error) {
	var trackURL string
	var routeURL string

	reTrack := regexp.MustCompile(
		`https://chalo\.com/app/api/vasudha/track/route-live-info/[^?]+`,
	)

	reRoute := regexp.MustCompile(
		`https://chalo\.com/app/api/scheduler_v4/v4/[^/]+/routedetailslive\?`,
	)

	opts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
		),
		chromedp.Flag("headless", false),
	)

	allocCtx, cancel := chromedp.NewExecAllocator(
		context.Background(),
		opts...,
	)
	defer cancel()

	ctx, cancel := chromedp.NewContext(allocCtx)
	defer cancel()

	if err := chromedp.Run(ctx, network.Enable()); err != nil {
		return "", "", err
	}

	done := make(chan struct{})

	chromedp.ListenTarget(ctx, func(ev interface{}) {
		req, ok := ev.(*network.EventRequestWillBeSent)
		if !ok {
			return
		}

		if req.Type != network.ResourceTypeXHR &&
			req.Type != network.ResourceTypeFetch {
			return
		}

		if match := reTrack.FindString(req.Request.URL); match != "" {
			trackURL = match
		}

		if match := reRoute.FindString(req.Request.URL); match != "" {
			routeURL = req.Request.URL
		}

		if trackURL != "" && routeURL != "" {
			select {
			case <-done:
			default:
				close(done)
			}
		}
	})

	err := chromedp.Run(ctx,
		chromedp.Navigate(
			fmt.Sprintf(
				"https://chalo.com/app/public-route/%s",
				BusCode,
			),
		),
		chromedp.WaitVisible("body"),
	)

	if err != nil {
		return "", "", err
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		return "", "", fmt.Errorf("timed out waiting for API requests")
	}
	fmt.Println(trackURL, routeURL)
	return trackURL, routeURL, nil
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
