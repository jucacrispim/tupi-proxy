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
	"io"
	"log"
	"net/http"
	"net/url"
)

var MissingConfigError error = errors.New("[tupi-proxy] Missing config")
var NoHostError error = errors.New("[tupi-proxy] Missing host config")
var BadHostError error = errors.New("[tupi-proxy] Bad host config")

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
	return nil
}

func Serve(w http.ResponseWriter, r *http.Request, conf *map[string]any) {
	c := (*conf)
	h := c["host"].(string) + r.URL.Path
	url, _ := url.Parse(h)

	r.Host = url.Host
	r.URL = url
	r.RequestURI = ""
	client := getHttpClient()
	resp, err := client.Do(r)
	if err != nil {
		log.Printf(err.Error())
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()
	for k, val := range resp.Header {
		for _, v := range val {
			w.Header().Add(k, v)
		}
	}

	for _, c := range resp.Cookies() {
		http.SetCookie(w, c)
	}

	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
}

type httpClient interface {
	Do(*http.Request) (*http.Response, error)
}

var testClient httpClient = nil

func getHttpClient() httpClient {
	// notest
	if testClient != nil {
		return testClient
	}
	return &http.Client{}
}
