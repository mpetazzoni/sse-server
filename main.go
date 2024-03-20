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

	limit := math.MaxInt
	count := math.MaxInt
	_, _ = fmt.Sscanf(request.FormValue("count"), "%d", &count)
	if count < 0 {
		srw.WriteHeader(http.StatusBadRequest)
		return
	}

	if count <= math.MaxInt-client.LastEventId {
		limit = client.LastEventId + count
	} else {
		limit = math.MaxInt
	}

	fmt.Printf("Starting stream for %s, %d -> %d ...\n", client.RemoteAddr, client.LastEventId, limit)

	srw.Header().Set("Access-Control-Allow-Origin", "*")
	srw.Header().Set("Access-Control-Allow-Methods", "GET")
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

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = defaultListenPort
	}

	// Read the token from AUTH_TOKEN_FILE, falling back to AUTH_TOKEN value.
	auth := NewAllowAllAuthValidator()
	token := os.Getenv("AUTH_TOKEN")
	tokenFile := os.Getenv("AUTH_TOKEN_FILE")
	if tokenFile != "" {
		bytes, err := os.ReadFile(tokenFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reading token from '%s': %v; ignoring.\n", tokenFile, err)
		} else {
			token = strings.TrimSpace(string(bytes))
		}
	}

	if token != "" {
		auth = NewTokenAuthValidator(token)
	}

	// Define the middleware stack
	authMiddleware := NewAuthMiddleware(auth)
	loggingMiddleware := NewLoggingMiddleware(os.Stdout)
	adapt := func(handler HandlerFunc) http.HandlerFunc {
		return AdaptHandler(handler, loggingMiddleware, authMiddleware)
	}

	ctx := NewHandlerContext()
	http.Handle("/status", adapt(ctx.StatusHandler))
	http.Handle("/stream", adapt(ctx.StreamHandler))

	fmt.Printf("Starting sse-server at :%s ...\n", port)
	if err := http.ListenAndServe(":"+port, nil); err != nil {
		panic(err)
	}
}
