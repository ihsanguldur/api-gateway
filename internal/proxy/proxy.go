package proxy

import (
	"net/http"
	"net/http/httputil"
	"net/url"
)

func NewSingleHostProxy(target string) (http.Handler, error) {
	u, err := url.Parse(target)
	if err != nil {
		return nil, err
	}

	return httputil.NewSingleHostReverseProxy(u), nil
}
