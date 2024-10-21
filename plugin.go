// Copyright 2024 Juca Crispim <juca@poraodojuca.net>

// This file is part of tupi-proxy.

// tupi-proxy is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// tupi-proxy is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with tupi-proxy. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
)

var MissingConfigError error = errors.New("[tupi-proxy] Missing config")
var NoHostError error = errors.New("[tupi-proxy] Missing host config")
var BadHostError error = errors.New("[tupi-proxy] Bad host config")
var BadPreserveHost error = errors.New("[tupi-proxy] Bad preserve host")

func Init(domain string, conf *map[string]any) error {
	c := (*conf)
	if c == nil {
		return MissingConfigError
	}

	h, exists := c["host"]
	if !exists {
		return NoHostError
	}

	_, ok := h.(string)
	if !ok {
		return BadHostError
	}
	_, err := url.Parse(h.(string))
	if err != nil {
		return BadHostError
	}

	if p, exists := c["preserveHost"]; exists {
		_, ok := p.(bool)
		if !ok {
			return BadPreserveHost
		}

	}

	return nil
}

func Serve(w http.ResponseWriter, r *http.Request, conf *map[string]any) {
	c := (*conf)
	bh := c["host"].(string)
	baseURL, _ := url.Parse(bh)
	origHost := r.Host
	host := ""
	if preserve, exists := c["preserveHost"]; exists {
		p := preserve.(bool)
		if p {
			host = origHost
		} else {
			host = baseURL.Host
		}
	}

	proxy := getHttpProxy(baseURL, host)
	proxy.ServeHTTP(w, r)
}

func rewriteRequest(req *httputil.ProxyRequest, url *url.URL, host string) {
	req.SetURL(url)
	req.Out.Host = host
}

// for tests
type httpProxy interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

var testProxy func(url *url.URL, host string) httpProxy

func getHttpProxy(url *url.URL, host string) httpProxy {
	// notest
	if testProxy != nil {
		return testProxy(url, host)
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(req *httputil.ProxyRequest) {
			rewriteRequest(req, url, host)
		},
	}
	return proxy
}
