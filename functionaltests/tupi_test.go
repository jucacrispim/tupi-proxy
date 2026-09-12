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
				r, _ := http.NewRequest("GET", "http://localhost:18080/the/path", nil)
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
				r, _ := http.NewRequest("POST", "http://localhost:18080",
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

	path := "/api/socks/waterfall-info?repo_name=someguy/repo-bla"
	client, err := NewWebSocketClient("ws://localhost:18080" + path)
	if err != nil {
		t.Fatalf("error creating websocket client %s", err.Error())
	}

	err = client.Handshake()
	if err != nil {
		t.Fatalf("error handshake %s", err.Error())
	}

	urifr, err := client.Recv()
	if err != nil {
		t.Fatalf("error recv uri %s", err.Error())
	}
	if string(urifr.Payload) != path {
		t.Fatalf("proxy dropped the query string: %s", string(urifr.Payload))
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

var serverCmd *exec.Cmd
var tupiCmd *exec.Cmd

func startProc(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	if cmd.Err != nil {
		panic(cmd.Err.Error())
	}
	err := cmd.Start()
	if err != nil {
		panic(err.Error())
	}
	return cmd
}

func stopProc(cmd *exec.Cmd) {
	if cmd == nil || cmd.Process == nil {
		return
	}
	cmd.Process.Kill()
	cmd.Wait()
}

func startServer() {
	serverCmd = startProc("./../build/testserver")
	tupiCmd = startProc("tupi", "-conf", "./../testdata/tupi-func.conf")
	time.Sleep(time.Millisecond * 200)
}

func stopServer() {
	stopProc(serverCmd)
	stopProc(tupiCmd)
	serverCmd = nil
	tupiCmd = nil
}

func startWSServer() {
	serverCmd = startProc("./../build/testwsserver", "-server")
	tupiCmd = startProc("tupi", "-conf", "./../testdata/tupi-func.conf")
	time.Sleep(time.Millisecond * 200)
}

func stopWSServer() {
	stopProc(serverCmd)
	stopProc(tupiCmd)
	serverCmd = nil
	tupiCmd = nil
}
