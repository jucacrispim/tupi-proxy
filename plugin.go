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
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

var MissingConfigError error = errors.New("[tupi-proxy] Missing config")
var NoHostError error = errors.New("[tupi-proxy] Missing host config")
var BadHostError error = errors.New("[tupi-proxy] Bad host config")
var BadPreserveHost error = errors.New("[tupi-proxy] Bad preserve host")
var InvalidScheme error = errors.New("Invalid scheme")

type wsProxy struct {
	destHost   string
	headerHost string
}

func (p *wsProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	hijacker, ok := w.(http.Hijacker)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("ResponseWriter not Hijacker")
		w.Write([]byte("Internal Server Error"))
		return
	}
	wsDest := strings.Replace(p.destHost, "http", "ws", 1)
	destURL := wsDest + r.URL.Path
	dest, _ := url.Parse(destURL)

	conn, _, err := hijacker.Hijack()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println("Error hijacking")
		w.Write([]byte("Internal Server Error"))
		return
	}
	defer conn.Close()
	outReq := r.Clone(r.Context())
	outReq.URL = dest
	outReq.Host = p.headerHost

	addr, _ := getHostPort(outReq.URL)
	destConn, err := dial("tcp", addr)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		log.Println(fmt.Sprintf("Error remote write: %s", err.Error()))
		w.Write([]byte("Internal Server Error"))
		return
	}

	defer destConn.Close()
	errCh := make(chan error, 2)
	copyIO := func(dest net.Conn, source net.Conn) {
		_, err := io.Copy(dest, source)
		if err != nil {
			log.Println(fmt.Sprintf("ws error: %s", err.Error()))
			errCh <- err
		}
	}

	outReq.Write(destConn)
	go copyIO(conn, destConn)
	go copyIO(destConn, conn)

	select {
	case <-errCh:
		log.Println("Closing ws conns")
	}

}

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
	destBaseURL, _ := url.Parse(bh)
	origHost := r.Host
	host := ""
	if preserve, exists := c["preserveHost"]; exists {
		p := preserve.(bool)
		if p {
			host = origHost
		} else {
			host = destBaseURL.Host
		}
	}

	var proxy httpProxy
	if !isWebSocket(r) {
		proxy = getHttpProxy(destBaseURL, host)
	} else {
		proxy = getWsProxy(destBaseURL, host)
	}
	proxy.ServeHTTP(w, r)
}

func rewriteRequest(req *httputil.ProxyRequest, url *url.URL, host string) {
	req.SetURL(url)
	req.Out.Host = host
}

func isWebSocket(r *http.Request) bool {
	isUpgrade := strings.ToLower(r.Header.Get("Connection")) == "upgrade"
	isWs := strings.ToLower(r.Header.Get("Upgrade")) == "websocket"

	return isUpgrade && isWs
}

func getHostPort(u *url.URL) (string, error) {
	hostname := u.Hostname()
	port := u.Port()
	if port == "" {
		switch u.Scheme {
		case "ws":
			port = "80"

		case "wss":
			port = "443"

		default:
			return "", InvalidScheme

		}
	}
	return fmt.Sprintf("%s:%s", hostname, port), nil
}

// for tests
type httpProxy interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

var testProxy func(url *url.URL, host string) httpProxy
var testConn net.Conn

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

func getWsProxy(url *url.URL, host string) httpProxy {
	// notest
	if testProxy != nil {
		return testProxy(url, host)
	}
	return &wsProxy{
		destHost:   url.String(),
		headerHost: host,
	}
}

var testDial func(n, a string) (net.Conn, error)

func dial(n, a string) (net.Conn, error) {
	// notest
	if testDial != nil {
		return testDial(n, a)
	}
	return net.Dial(n, a)
}
