package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

func main() {
	input := flag.String("f", "", "path to list of urls or ip addresses, defaults to stdin if left blank")
	concurrent := flag.Int("c", 10, "number of concurrent requests to make")
	timeout := flag.Int("t", 5, "timeout in seconds")
	redirect := flag.Bool("r", false, "follow redirects")
	showUrls := flag.Bool("u", false, "show urls in output")
	flag.Parse()

	var f io.ReadCloser
	if *input == "" {
		f = os.Stdin
	} else {
		file, err := os.Open(*input)
		if err != nil {
			fmt.Fprintln(os.Stderr, "unable to open input file:", err)
			os.Exit(1)
		}
		f = file
	}
	defer f.Close()

	work := make(chan string)
	go func() {
		s := bufio.NewScanner(f)
		for s.Scan() {
			work <- s.Text()
		}
		if s.Err() != nil {
			fmt.Fprintln(os.Stderr, "error while scanning input:", s.Err())
		}
		close(work)
	}()

	client := &http.Client{
		Timeout: time.Second * time.Duration(*timeout),
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}
	if !*redirect {
		client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		}

	}

	wg := &sync.WaitGroup{}
	for i := 0; i < *concurrent; i++ {
		wg.Add(1)
		go getCommon(client, work, wg, *showUrls)
	}
	wg.Wait()
}

func getCommon(client *http.Client, work chan string, wg *sync.WaitGroup, showUrls bool) {
	for url := range work {
		if !strings.HasPrefix(url, "https://") {
			url = "https://" + url
		}
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		req.Close = true
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		io.Copy(ioutil.Discard, resp.Body)
		resp.Body.Close()
		if resp.TLS == nil || len(resp.TLS.PeerCertificates) == 0 || resp.TLS.PeerCertificates[0].Subject.CommonName == "" {
			continue
		}
		if showUrls {
			fmt.Println(url, resp.TLS.PeerCertificates[0].Subject.CommonName)
		} else {
			fmt.Println(resp.TLS.PeerCertificates[0].Subject.CommonName)
		}
	}
	wg.Done()
}
