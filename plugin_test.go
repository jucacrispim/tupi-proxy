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
	"io"
	"net/http"
	"net/http/httptest"
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

type BadClient struct{}

func (c *BadClient) Do(r *http.Request) (*http.Response, error) {
	return nil, errors.New("some error")
}

type GoodClient struct{}

func (c *GoodClient) Do(r *http.Request) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: 200,
		Body:       io.NopCloser(strings.NewReader("something")),
	}
	return resp, nil
}

func TestServe(t *testing.T) {
	type validateFn func(w *httptest.ResponseRecorder)
	var tests = []struct {
		name     string
		r        *http.Request
		client   httpClient
		validate validateFn
	}{
		{
			"bad request to host",
			func() *http.Request {
				r, _ := http.NewRequest("GET", "/bla/x", nil)
				return r
			}(),
			&BadClient{},
			func(w *httptest.ResponseRecorder) {
				if w.Code != http.StatusInternalServerError {
					t.Fatalf("Invalid status code %d", w.Code)
				}
			},
		},
		{
			"request ok",
			func() *http.Request {
				r, _ := http.NewRequest("GET", "/bla/x", nil)
				return r
			}(),
			&GoodClient{},
			func(w *httptest.ResponseRecorder) {
				if w.Code != http.StatusOK {
					t.Fatalf("Invalid status code %d", w.Code)
				}
				b := string(w.Body.Bytes())
				if b != "something" {
					t.Fatalf("Bad body %s", b)
				}
			},
		},
	}

	defer func() {
		testClient = nil
	}()

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			testClient = test.client
			w := httptest.NewRecorder()
			conf := map[string]any{"host": "http://localhost:8080"}
			Serve(w, test.r, &conf)
			test.validate(w)
		})
	}
}
