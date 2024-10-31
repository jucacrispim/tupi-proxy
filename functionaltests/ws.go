package functionaltests

// notest

import (
	"bufio"
	"crypto/sha1"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"strings"
)

const (
	OpcodeText   = 0x00
	OpcodeBinary = 0x01
	OpcodeClose  = 0x08
	OpcodePing   = 0x09
	OpcodePong   = 0x0A
)

type Frame struct {
	Opcode   byte
	Len      uint
	Payload  []byte
	Mask     []byte
	IsFinal  bool
	IsMasked bool
}

type WebSocket struct {
	Conn net.Conn
}

func (ws *WebSocket) Send(fr *Frame) error {
	data := ws.WireEncode(fr)
	_, err := ws.Conn.Write(data)
	return err
}

func (ws *WebSocket) Recv() (*Frame, error) {

	for {
		fr, err := ws.WireDecode()

		if err != nil {
			return &Frame{}, err
		}

		switch fr.Opcode {
		case OpcodeClose:
			return &Frame{}, io.EOF

		case OpcodePing:
			fr.Opcode = OpcodePong
			err := ws.Send(fr)
			if err != nil {
				return &Frame{}, err
			}

		default:
			return fr, nil

		}
	}

}

func (ws *WebSocket) Close() error {
	msg := []byte("close connection")
	fr := Frame{
		Opcode:  OpcodeClose,
		Payload: msg,
		Len:     uint(len(msg)),
		IsFinal: true,
	}
	ws.Send(&fr)
	return ws.Conn.Close()
}

func (ws *WebSocket) WireEncode(fr *Frame) []byte {
	data := make([]byte, 2)
	if fr.IsFinal {
		data[0] = 0x00
	} else {
		data[0] = 0x80
	}
	data[0] |= fr.Opcode

	l := len(fr.Payload)

	if l <= 125 {
		data[1] = byte(l)

	} else if float64(l) < math.Pow(2, 16) {
		data[1] = byte(126)
		s := make([]byte, 2)
		binary.BigEndian.PutUint16(s, uint16(l))
		data = append(data, s...)
	} else {
		data[1] = byte(127)
		s := make([]byte, 8)
		binary.BigEndian.PutUint64(s, uint64(l))
		data = append(data, s...)
	}

	if len(fr.Mask) == 4 {
		data[1] = 0x80 | data[1]
		data = append(data, fr.Mask...)
		xOR(fr.Payload, fr.Mask)
	}
	data = append(data, fr.Payload...)
	return data
}

func (ws *WebSocket) WireDecode() (*Frame, error) {
	fr := Frame{}
	d := make([]byte, 2)
	_, err := ws.Conn.Read(d)
	if err != nil {
		return nil, err
	}

	final := (d[0] & 0x80) == 0x00
	opcode := d[0] & 0x0F
	isMasked := (d[1] & 0x80) == 0x80
	len := d[1] & 0x7F
	l := uint(len)

	fr.Opcode = opcode
	fr.IsFinal = final
	fr.IsMasked = isMasked

	if l == 126 {
		d := make([]byte, 2)
		_, err := ws.Conn.Read(d)
		if err != nil {
			return nil, err
		}
		l = uint(binary.BigEndian.Uint16(d))
	} else if l == 127 {
		d := make([]byte, 8)
		_, err := ws.Conn.Read(d)
		if err != nil {
			return nil, err
		}
		l = uint(binary.BigEndian.Uint64(d))
	}

	fr.Len = l

	mask := make([]byte, 4)
	if isMasked {
		_, err = ws.Conn.Read(mask)
		if err != nil {
			return nil, err
		}
	}

	payload := make([]byte, l)
	_, err = ws.Conn.Read(payload)

	if isMasked {
		xOR(payload, mask)
		fr.Mask = mask

	}
	fr.Payload = payload
	return &fr, nil
}

type WebSocketClient struct {
	WebSocket
	Url *url.URL
}

func (ws *WebSocketClient) Handshake() error {
	hash := getSecHashClient()
	req := &http.Request{
		URL:    ws.Url,
		Header: make(http.Header),
	}
	req.Header.Set("Upgrade", "websocket")
	req.Header.Set("Connection", "upgrade")
	req.Header.Set("Sec-WebSocket-Accept", hash)

	err := req.Write(ws.WebSocket.Conn)
	if err != nil {
		return err
	}
	reader := bufio.NewReaderSize(ws.Conn, 4096)
	resp, err := http.ReadResponse(reader, req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusSwitchingProtocols {
		return errors.New("Server does not support websockets")
	}

	if strings.ToLower(resp.Header.Get("Upgrade")) != "websocket" ||
		strings.ToLower(resp.Header.Get("Connection")) != "upgrade" {
		return errors.New("Invalid response")
	}
	return nil
}

func (ws *WebSocketClient) Send(fr *Frame) error {
	fr.Mask = getMask()
	return ws.WebSocket.Send(fr)
}

func NewWebSocketClient(rawURL string) (*WebSocketClient, error) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return &WebSocketClient{}, nil
	}

	hostPort, err := getHostPort(u)
	if err != nil {
		return &WebSocketClient{}, err
	}

	conn, err := net.Dial("tcp", hostPort)
	if err != nil {
		return &WebSocketClient{}, err
	}
	ws := WebSocketClient{
		WebSocket: WebSocket{
			Conn: conn,
		},
		Url: u,
	}

	return &ws, nil
}

type WebSocketServer struct {
	WebSocket
	Header http.Header
}

func (ws *WebSocketServer) Handshake() error {
	secKey := ws.Header.Get("Sec-WebSocket-Key")
	hash := getSecHashServer(secKey)
	headers := []string{
		"HTTP/1.1 101 Switching Protocols",
		"Upgrade: websocket",
		"Connection: upgrade",
		"Sec-WebSocket-Accept: " + hash,
		"",
		"",
	}
	_, err := ws.Conn.Write([]byte(strings.Join(headers, "\r\n")))
	return err
}

func (ws *WebSocketServer) Recv() (*Frame, error) {
	fr, err := ws.WebSocket.Recv()

	if err != nil {
		return fr, err
	}
	if !fr.IsMasked {
		return &Frame{}, errors.New("Clients must mask the payload")
	}
	return fr, err
}

func (ws *WebSocketServer) Echo() error {
	for {
		fr, err := ws.Recv()
		if err != nil && errors.Is(err, io.EOF) {
			log.Println("Connection closed")
			return nil
		}

		if err != nil {
			log.Println(err.Error())
			return err
		}

		fr.Mask = []byte{}
		fr.IsMasked = false

		err = ws.Send(fr)
		if err != nil {
			log.Println(err.Error())
			return err
		}
	}
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
			return "", errors.New("Invalid scheme " + u.Scheme)

		}
	}
	return fmt.Sprintf("%s:%s", hostname, port), nil
}

func xOR(data []byte, mask []byte) {
	for i := 0; i < len(data); i++ {
		data[i] ^= mask[i%4]
	}
}

func getMask() []byte {
	var m []byte
	for i := 0; i < 4; i++ {
		m = append(m, byte(rand.Intn(255)))
	}
	return m
}

func getSecHashClient() string {
	var h []byte
	for i := 0; i < 16; i++ {
		h = append(h, byte(rand.Intn(127-32)+32))
	}
	return base64.StdEncoding.EncodeToString(h)
}

func getSecHashServer(sk string) string {
	h := sha1.New()
	h.Write([]byte(sk))
	h.Write([]byte("258EAFA5-E914-47DA-95CA-C5AB0DC85B11"))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}
