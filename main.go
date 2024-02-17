package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

const defaultListenPort = "8080"
const messageIdPrefix = "message-"
const randomStringLength = 16

type Client struct {
	RemoteAddr  string    `json:"remote"`
	ConnectedAt time.Time `json:"connectedAt"`
	LastEventId int       `json:"lastEventId"`
}

type StreamResponseWriter struct {
	http.ResponseWriter
	controller *http.ResponseController
	statusCode int
}

func (srw *StreamResponseWriter) WriteHeader(statusCode int) {
	srw.statusCode = statusCode
	srw.ResponseWriter.WriteHeader(statusCode)
}

func (srw *StreamResponseWriter) WriteEvent(id string, data string) error {
	_, err := fmt.Fprintf(srw, "id: %s\n", id)
	if err != nil {
		return err
	}

	sc := bufio.NewScanner(strings.NewReader(data))
	for sc.Scan() {
		_, err = fmt.Fprintf(srw, "data: %s\n", sc.Text())
		if err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(srw, "\n")
	if err != nil {
		return err
	}

	err = srw.controller.Flush()
	if err != nil {
		return err
	}

	return nil
}

func NewStatusResponseWriter(writer http.ResponseWriter) *StreamResponseWriter {
	return &StreamResponseWriter{
		ResponseWriter: writer,
		controller:     http.NewResponseController(writer),
		statusCode:     http.StatusOK,
	}
}

type HandlerContext struct {
	clients map[string]*Client
}

func NewHandlerContext() *HandlerContext {
	return &HandlerContext{clients: make(map[string]*Client)}
}

type HandlerFunc func(srw *StreamResponseWriter, request *http.Request)

func (f HandlerFunc) ServeHTTP(srw *StreamResponseWriter, request *http.Request) {
	f(srw, request)
}

func (ctx *HandlerContext) StreamHandler(srw *StreamResponseWriter, request *http.Request) {
	defer func() {
		delete(ctx.clients, request.RemoteAddr)
		fmt.Printf("Client %s closed connection.\n", request.RemoteAddr)
	}()

	client := &Client{
		RemoteAddr:  request.RemoteAddr,
		ConnectedAt: time.Now(),
		LastEventId: 1,
	}
	ctx.clients[request.RemoteAddr] = client

	lastEventId := request.Header.Get("Last-Event-Id")
	_, _ = fmt.Sscanf(lastEventId, "message-%d", &client.LastEventId)

	count := math.MaxInt
	_, _ = fmt.Sscanf(request.FormValue("count"), "%d", &count)
	limit := client.LastEventId + count

	fmt.Printf("Starting stream for %s @ %d...\n", client.RemoteAddr, client.LastEventId)

	srw.Header().Set("Content-Type", "text/event-stream")
	srw.Header().Set("Cache-Control", "no-cache")
	srw.Header().Set("Connection", "keep-alive")
	if count != math.MaxInt {
		srw.Header().Set("X-Expected-Events", strconv.Itoa(count))
	}
	srw.WriteHeader(http.StatusOK)

	err := srw.WriteEvent("hello", fmt.Sprintf("Hello, %s!", client.RemoteAddr))
	if err != nil {
		return
	}

	for ; client.LastEventId < limit; client.LastEventId++ {
		time.Sleep(1 * time.Second)

		random := GenerateRandomString(client.LastEventId, randomStringLength)
		err := srw.WriteEvent(
			fmt.Sprintf("%s%d", messageIdPrefix, client.LastEventId),
			fmt.Sprintf("{\n  \"time\": %d,\n  \"random\": \"%s\"\n}", time.Now().Unix(), random))
		if err != nil {
			return
		}
	}
}

func (ctx *HandlerContext) StatusHandler(srw *StreamResponseWriter, request *http.Request) {
	body, err := json.Marshal(ctx.clients)
	if err != nil {
		srw.WriteHeader(http.StatusInternalServerError)
		return
	}

	srw.Header().Set("Content-Type", "application/json")
	_, _ = srw.Write(body)
}

func loggerMiddleware(f HandlerFunc) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		srw := NewStatusResponseWriter(writer)
		start := time.Now()

		defer func() {
			fmt.Printf("%s %s %s %d (%v)\n",
				start.Format(time.RFC3339),
				request.Method,
				request.URL,
				srw.statusCode,
				time.Since(start))
		}()
		f.ServeHTTP(srw, request)
	}
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultListenPort
	}

	ctx := NewHandlerContext()
	http.HandleFunc("/status", loggerMiddleware(ctx.StatusHandler))
	http.HandleFunc("/stream", loggerMiddleware(ctx.StreamHandler))

	fmt.Printf("Starting sse-server at :%s ...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic(err)
	}
}
