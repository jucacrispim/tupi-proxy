package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"

	"github.com/jucacrispim/tupi-proxy/functionaltests"
)

func wsCli() {

	ws, err := functionaltests.NewWebSocketClient("ws://localhost:8082")
	if err != nil {
		panic(err.Error())
	}

	err = ws.Handshake()
	if err != nil {
		panic(err.Error())
	}

	var msg string
	for {
		fmt.Print(": ")
		_, err := fmt.Scanln(&msg)
		if err != nil {
			panic(err.Error())
		}

		frame := functionaltests.Frame{
			Payload: []byte(msg),
			IsFinal: true,
			Opcode:  functionaltests.OpcodeText,
		}
		ws.Send(&frame)
		resp, err := ws.Recv()
		if err != nil {
			panic(err.Error())
		}
		println(fmt.Sprintf("Server echoed %s", string(resp.Payload)))

	}

}
func wsHandler(w http.ResponseWriter, r *http.Request) {
	h, ok := w.(http.Hijacker)
	if !ok {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	conn, _, err := h.Hijack()
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	ws := functionaltests.WebSocketServer{
		WebSocket: functionaltests.WebSocket{
			Conn: conn,
		},
		Header: r.Header,
	}

	defer ws.Close()

	err = ws.Handshake()
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = ws.Echo()
	if err != nil {
		log.Println(err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

func main() {
	server := flag.Bool("server", false, "start the server")
	client := flag.Bool("client", false, "start the client")

	flag.Parse()

	if !*server && !*client {
		panic("one of server or client must be true")
	}

	if *server && *client {
		panic("only one of server and client can be true")
	}
	if *server {
		log.Fatal(http.ListenAndServe(":8081", http.HandlerFunc(wsHandler)))
	} else {
		wsCli()
	}
}
