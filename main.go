/* ProxyGet
 */

package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"time"
)

var proxyString string
var rawURL string
var debug = false

func main() {
	if len(os.Args) != 3 && len(os.Args) != 4 {
		fmt.Println("Usage: ", os.Args[0], "http://proxy-host:port http://host:port/page")
		os.Exit(1)
	}
	proxyString = os.Args[1]
	rawURL = os.Args[2]

	if len(os.Args) == 4 && os.Args[3] == "debug" {
		debug = true
	}

	fmt.Printf("%s %s\n", proxyString, rawURL)

	handler := NewReverseProxyViaProxy(rawURL, proxyString)

	http.HandleFunc("/", handler)

	log.Fatal(http.ListenAndServe(":8080", nil))

}

func NewReverseProxyViaProxy(target string, proxy string) func(w http.ResponseWriter, r *http.Request) {
	targetURL, err := url.Parse(target)
	checkError(err)

	proxyURL, err := url.Parse(proxy)
	checkError(err)

	transport := &http.Transport{
		Proxy:               http.ProxyURL(proxyURL),
		DisableKeepAlives:   true,
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true},
		MaxConnsPerHost:     100,
		MaxIdleConnsPerHost: 100,
		MaxIdleConns:        100,
		//IdleConnTimeout:     90 * time.Second,
		//ExpectContinueTimeout: 5 * time.Second,
		//TLSHandshakeTimeout:   10 * time.Second,
		//ResponseHeaderTimeout: 90 * time.Second,
		DialContext: (&net.Dialer{
			//Timeout:   90 * time.Second,
			KeepAlive: 10 * time.Second,
		}).DialContext,
	}

	reverseProxy := httputil.NewSingleHostReverseProxy(targetURL)
	reverseProxy.FlushInterval = -1
	reverseProxy.Transport = transport
	reverseProxy.Director = func(req *http.Request) {
		req.URL.Scheme = targetURL.Scheme
		req.URL.Host = targetURL.Host
		req.Host = proxyURL.Host
		if _, ok := req.Header["User-Agent"]; !ok {
			// explicitly disable User-Agent so it's not set to default value
			req.Header.Set("User-Agent", "")
		}
		if debug {
			dump, err := httputil.DumpRequestOut(req, true)
			if err != nil {
				fmt.Print("dump error", err)
			}
			fmt.Printf("%q\n", dump)
		}
		req.Close = true
	}

	reverseProxy.ModifyResponse = func(response *http.Response) error {
		if debug && response.StatusCode != 200 {
			fmt.Printf("status code %s %d %s\n", time.Now().String(), response.StatusCode, response.Request.URL)
		}
		return nil
	}
	reverseProxy.ErrorHandler = func(writer http.ResponseWriter, request *http.Request, err error) {
		if err != nil {
			fmt.Println("reqres err", err)
		}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		reverseProxy.ServeHTTP(w, r)
	}
}

func checkError(err error) {
	if err != nil {
		if err == io.EOF {
			return
		}
		fmt.Println("Fatal error ", err.Error())
		os.Exit(1)
	}
}
