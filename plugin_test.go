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
	"errors"
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
