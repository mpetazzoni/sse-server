package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const messageIdPrefix = "message-"
const randomStringLength = 16

type Client struct {
	RemoteAddr  string    `json:"remote"`
	ConnectedAt time.Time `json:"connectedAt"`
	LastEventId int       `json:"lastEventId"`
}

func emit(writer io.Writer, rc *http.ResponseController, id string, data string) error {
	_, err := fmt.Fprintf(writer, "id: %s\n", id)
	if err != nil {
		return err
	}

	sc := bufio.NewScanner(strings.NewReader(data))
	for sc.Scan() {
		_, err = fmt.Fprintf(writer, "data: %s\n", sc.Text())
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(writer, "\n")
	if err != nil {
		return err
	}

	err = rc.Flush()
	if err != nil {
		return err
	}

	return nil
}

type HandlerContext struct {
	clients map[string]*Client
}

func NewHandlerContext() *HandlerContext {
	return &HandlerContext{clients: make(map[string]*Client)}
}

func (ctx *HandlerContext) StreamHandler(writer http.ResponseWriter, request *http.Request) {
	defer func() {
		delete(ctx.clients, request.RemoteAddr)
		fmt.Printf("Client %s closed connection.\n", request.RemoteAddr)
	}()

	client := &Client{
		RemoteAddr:  request.RemoteAddr,
		ConnectedAt: time.Now(),
		LastEventId: 0,
	}
	ctx.clients[request.RemoteAddr] = client

	lastEventId := request.Header.Get("Last-Event-Id")
	_, _ = fmt.Sscanf(lastEventId, "message-%d", &client.LastEventId)

	fmt.Printf("Handling incoming request from %s @ %d...\n", client.RemoteAddr, client.LastEventId)

	rc := http.NewResponseController(writer)
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	writer.WriteHeader(http.StatusOK)

	if err := emit(writer, rc, "hello", fmt.Sprintf("Hello, %s!", client.RemoteAddr)); err != nil {
		return
	}

	for ; ; client.LastEventId++ {
		random := GenerateRandomString(client.LastEventId, randomStringLength)
		err := emit(writer, rc,
			fmt.Sprintf("%s%d", messageIdPrefix, client.LastEventId),
			fmt.Sprintf("{\n  \"time\": %d,\n  \"random\": \"%s\"\n}", time.Now().Unix(), random))
		if err != nil {
			return
		}
		time.Sleep(1 * time.Second)
	}
}

func (ctx *HandlerContext) StatusHandler(writer http.ResponseWriter, request *http.Request) {
	body, err := json.Marshal(ctx.clients)
	if err != nil {
		writer.WriteHeader(http.StatusInternalServerError)
		return
	}

	writer.Header().Set("Content-Type", "application/json")
	_, _ = writer.Write(body)
}

func main() {
	fmt.Println("Hello, world!")

	ctx := NewHandlerContext()
	http.HandleFunc("/stream", ctx.StreamHandler)
	http.HandleFunc("/status", ctx.StatusHandler)
	if err := http.ListenAndServe(":8080", nil); err != nil {
		panic(err)
	}
}
