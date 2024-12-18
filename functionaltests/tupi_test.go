package functionaltests

import (
	"io"
	"net/http"
	"os/exec"
	"strings"
	"testing"
	"time"
)

func TestHttpProxy(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	type validateResponse func(*http.Response)

	startServer()
	defer stopServer()

	var tests = []struct {
		name     string
		request  *http.Request
		validate validateResponse
	}{
		{
			"get request",
			func() *http.Request {
				r, _ := http.NewRequest("GET", "http://localhost:8080/the/path", nil)
				return r
			}(),
			func(r *http.Response) {
				if r.StatusCode != 200 {
					t.Fatalf("Bad status %d", r.StatusCode)
				}
				defer r.Body.Close()
				b := make([]byte, r.ContentLength)
				r.Body.Read(b)
				if string(b) != "Method was: GET\nPath was: /the/path" {
					t.Fatalf("Bad body %s", string(b))
				}

			},
		},
		{
			"post request",
			func() *http.Request {
				r, _ := http.NewRequest("POST", "http://localhost:8080",
					io.NopCloser(strings.NewReader("The body")))
				return r
			}(),
			func(r *http.Response) {
				if r.StatusCode != 200 {
					t.Fatalf("Bad status %d", r.StatusCode)
				}
				defer r.Body.Close()
				b, _ := io.ReadAll(r.Body)
				if string(b) != "Method was: POST\nPath was: /\nBody was: The body" {
					t.Fatalf("Bad body %s", string(b))
				}
				if r.Header.Get("A-CUSTOM") != "THING" {
					t.Fatalf("bad header custom %s", r.Header.Get("A-CUSTOM"))
				}

			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			c := http.Client{}
			resp, err := c.Do(test.request)

			if err != nil {
				t.Fatal(err)
			}
			test.validate(resp)

		})
	}
}

func TestWSProxy(t *testing.T) {
	if testing.Short() {
		t.Skip()
	}

	type validateResponse func(*http.Response)

	startWSServer()
	defer stopWSServer()

	client, err := NewWebSocketClient("ws://localhost:8080")
	if err != nil {
		t.Fatalf("error creating websocket client %s", err.Error())
	}

	err = client.Handshake()
	if err != nil {
		t.Fatalf("error handshake %s", err.Error())
	}

	msg := "testing ws"
	fr := Frame{
		Payload: []byte(msg),
		IsFinal: true,
		Opcode:  OpcodeText,
	}
	err = client.Send(&fr)
	if err != nil {
		t.Fatalf("error sending msg %s", err.Error())
	}

	rfr, err := client.Recv()
	if err != nil {
		t.Fatalf("error recv %s", err.Error())
	}

	if rfr.Opcode != OpcodeText {
		t.Fatalf("bad opcode %b", rfr.Opcode)
	}
	if string(rfr.Payload) != msg {
		t.Fatalf("bad response %s", string(rfr.Payload))
	}

}

func startServer() {
	cmd := exec.Command("./../build/testserver")
	if cmd.Err != nil {
		panic(cmd.Err.Error())
	}
	err := cmd.Start()
	if err != nil {
		panic(err.Error())
	}

	cmd = exec.Command("tupi", "-conf", "./../testdata/tupi-func.conf")
	if cmd.Err != nil {
		panic(cmd.Err.Error())
	}
	err = cmd.Start()
	if err != nil {
		panic(err.Error())
	}
	time.Sleep(time.Millisecond * 200)
}

func stopServer() {
	exec.Command("killall", "testserver").Run()
	exec.Command("killall", "tupi").Run()
}

func startWSServer() {
	cmd := exec.Command("./../build/testwsserver", "-server")
	if cmd.Err != nil {
		panic(cmd.Err.Error())
	}
	err := cmd.Start()
	if err != nil {
		panic(err.Error())
	}

	cmd = exec.Command("tupi", "-conf", "./../testdata/tupi-func.conf")
	if cmd.Err != nil {
		panic(cmd.Err.Error())
	}
	err = cmd.Start()
	if err != nil {
		panic(err.Error())
	}
	time.Sleep(time.Millisecond * 200)
}

func stopWSServer() {
	exec.Command("killall", "testwsserver").Run()
	exec.Command("killall", "tupi").Run()
}
