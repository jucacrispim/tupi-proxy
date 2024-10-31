// Copyright 2024 Juca Crispim <juca@poraodojuca.net>

// This file is part of tupi-cgi.

// tupi-cgi is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.

// tupi-cgi is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU General Public License for more details.

// You should have received a copy of the GNU Affero General Public License
// along with tupi-cgi. If not, see <http://www.gnu.org/licenses/>.

package main

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
)

func TestInit(t *testing.T) {
	var tests = []struct {
		name string
		conf map[string]any
		err  error
	}{
		{
			"missing config",
			nil,
			MissingConfigError,
		},
		{
			"missing host",
			map[string]any{},
			NoHostError,
		},
		{
			"bad host",
			map[string]any{"host": 1},
			BadHostError,
		},
		{
			"malformed host",
			map[string]any{"host": "bad://sdf.xx:jj?"},
			BadHostError,
		},
		{
			"ok",
			map[string]any{"host": "http://host.bla"},
			nil,
		},
		{
			"bad preserve host",
			map[string]any{"host": "http://host.bla", "preserveHost": "x"},
			BadPreserveHost,
		},
		{
			"ok preserve host",
			map[string]any{"host": "http://host.bla", "preserveHost": true},
			nil,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := Init("some.domain", &test.conf)
			if !errors.Is(err, test.err) {
				t.Fatalf("bad err %s", err.Error())
			}
		})
	}
}

type myProxy struct {
	url  *url.URL
	host string
	pr   *httputil.ProxyRequest
}

func (p *myProxy) Rewrite(r *httputil.ProxyRequest) {
	rewriteRequest(r, p.url, p.host)
}

func (p *myProxy) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	outreq := r.Clone(ctx)
	pr := &httputil.ProxyRequest{
		In:  r,
		Out: outreq,
	}
	p.Rewrite(pr)
	p.pr = pr
}

type bufferConn struct {
	net.TCPConn
	b bytes.Buffer
}

func (bc *bufferConn) Read(b []byte) (int, error) {
	return bc.b.Read(b)
}

func (bc *bufferConn) Write(b []byte) (int, error) {
	return bc.b.Write(b)
}

func (bc *bufferConn) WriteTo(w io.Writer) (n int64, err error) {
	total := 0
	for {
		var b = make([]byte, 10)
		r, err := bc.Read(b)
		if err != nil {
			return int64(total), err
		}
		if r > 0 {
			w.Write(b)
			total += r
		}
	}
}

type myHijacker struct {
	httptest.ResponseRecorder
	inConn    net.Conn
	destConn  net.Conn
	withError bool
}

func (h *myHijacker) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h.withError {
		return nil, nil, errors.New("Bad hijack")
	}
	return h.inConn, nil, nil
}

func newHijacker(withError bool) *myHijacker {
	var b []byte
	buff := bytes.NewBuffer(b)
	destConn := bufferConn{
		TCPConn: net.TCPConn{},
		b:       *buff,
	}
	var ib []byte
	inbuff := bytes.NewBuffer(ib)
	inConn := bufferConn{
		TCPConn: net.TCPConn{},
		b:       *inbuff,
	}
	return &myHijacker{
		ResponseRecorder: *httptest.NewRecorder(),
		inConn:           &inConn,
		destConn:         &destConn,
		withError:        withError,
	}
}

func TestServe(t *testing.T) {
	type validateFn func(w *httptest.ResponseRecorder)
	var tests = []struct {
		name string
		conf map[string]any
	}{
		{
			"request preserve root",
			map[string]any{"host": "http://localhost:8000", "preserveHost": true},
		},
		{
			"request don't preserve root",
			map[string]any{"host": "http://localhost:8000", "preserveHost": false},
		},
	}

	defer func() {
		testProxy = nil
	}()

	for _, test := range tests {
		var p *myProxy
		t.Run(test.name, func(t *testing.T) {
			testProxy = func(url *url.URL, host string) httpProxy {
				p = &myProxy{
					url:  url,
					host: host,
				}
				return p
			}
			w := httptest.NewRecorder()
			r, _ := http.NewRequest("GET", "/", nil)
			r.Host = "https://the.site.net"
			conf := test.conf
			Serve(w, r, &conf)

			outhost := p.pr.Out.Host
			if conf["preserveHost"].(bool) && strings.Index(outhost, "localhost") >= 0 {
				t.Fatalf("bad preserve host %s", outhost)

			}
		})
	}
}

func TestServeWS(t *testing.T) {

	defer func() {
		testDial = nil
	}()

	type validateFn func(http.ResponseWriter)

	var tests = []struct {
		name     string
		writerFn func() http.ResponseWriter
		conf     map[string]any
		validate validateFn
		serveFn  func(w http.ResponseWriter, r *http.Request, c *map[string]any)
	}{
		{
			"test writer not hijacker",
			func() http.ResponseWriter { return httptest.NewRecorder() },
			map[string]any{"host": "http://my-host.nada"},
			func(w http.ResponseWriter) {
				tw := w.(*httptest.ResponseRecorder)
				if tw.Code != 500 {
					t.Fatalf("Bad code %d", tw.Code)
				}
			},
			Serve,
		},
		{
			"test bad hijack",
			func() http.ResponseWriter { return newHijacker(true) },
			map[string]any{"host": ""},
			func(w http.ResponseWriter) {
				tw := w.(*myHijacker)
				if tw.Code != 500 {
					t.Fatalf("bad code %d", tw.Code)
				}
			},
			Serve,
		},
		{
			"test bad dial",
			func() http.ResponseWriter {
				h := newHijacker(false)
				testDial = func(n, a string) (net.Conn, error) {
					return nil, errors.New("Bad dial")
				}
				return h
			},
			map[string]any{"host": "http://nada.bla"},
			func(w http.ResponseWriter) {
				tw := w.(*myHijacker)
				if tw.Code != 500 {
					t.Fatalf("bad code %d", tw.Code)
				}
			},
			Serve,
		},
		{
			"test ok",
			func() http.ResponseWriter {
				h := newHijacker(false)
				testDial = func(n, a string) (net.Conn, error) {
					return h.destConn, nil
				}
				return h
			},
			map[string]any{"host": "http://localhost"},
			func(w http.ResponseWriter) {
				tw := w.(*myHijacker)
				var r []byte

				for {
					r, _ = io.ReadAll(tw.destConn)
					if len(r) >= 1 {
						if strings.Index(string(r), "Upgrade: websocket") < 0 {
							t.Fatalf("Bad headers")
						}
						break
					}
				}

				tw.destConn.Close()
				tw.inConn.Close()
			},
			func(w http.ResponseWriter, r *http.Request, c *map[string]any) {
				go Serve(w, r, c)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, _ := http.NewRequest("GET", "/", nil)
			r.Header.Set("Connection", "upgrade")
			r.Header.Set("Upgrade", "websocket")
			writer := test.writerFn()
			test.serveFn(writer, r, &test.conf)
			test.validate(writer)
		})
	}
}

func TestGetHostPort(t *testing.T) {
	var tests = []struct {
		name         string
		url          *url.URL
		expectedAddr string
		err          error
	}{
		{
			"url with host and port",
			func() *url.URL {
				u, _ := url.Parse("ws://localhost:8080")
				return u
			}(),
			"localhost:8080",
			nil,
		},
		{
			"ws url",
			func() *url.URL {
				u, _ := url.Parse("ws://localhost")
				return u
			}(),
			"localhost:80",
			nil,
		},
		{
			"wss url",
			func() *url.URL {
				u, _ := url.Parse("wss://localhost")
				return u
			}(),
			"localhost:443",
			nil,
		},
		{
			"bad scheme",
			func() *url.URL {
				u, _ := url.Parse("wxs://localhost")
				return u
			}(),
			"",
			InvalidScheme,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, err := getHostPort(test.url)
			if err != nil && !errors.Is(err, test.err) {
				t.Fatalf("Bad err %s", err.Error())
			}

			if r != test.expectedAddr {
				t.Fatalf("bad addr %s", r)
			}
		})
	}
}
