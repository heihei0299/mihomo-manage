package infra

import (
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"
)

var (
	downloadClient *http.Client
	once           sync.Once
)

func getDownloadClient() *http.Client {
	once.Do(func() {
		transport := &http.Transport{
			Proxy: nil,
		}
		if p := os.Getenv("MIHOMO_DOWNLOAD_PROXY"); p != "" {
			if parsed, err := url.Parse(p); err == nil {
				transport.Proxy = http.ProxyURL(parsed)
			}
		}
		downloadClient = &http.Client{
			Transport: transport,
			Timeout:   10 * time.Minute,
		}
	})
	return downloadClient
}

// SetDownloadProxy configures the download proxy. It resets the lazy
// initializer so that subsequent calls to getDownloadClient pick up the
// new proxy value.
func SetDownloadProxy(proxyURL string) {
	once = sync.Once{}
	os.Setenv("MIHOMO_DOWNLOAD_PROXY", proxyURL)
	downloadClient = nil
	getDownloadClient()
}
