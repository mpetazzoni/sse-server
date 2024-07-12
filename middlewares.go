package main

import (
	"fmt"
	"io"
	"net/http"
	"time"
)

type HttpMiddleware func(handlerFunc HandlerFunc) HandlerFunc

func NewLoggingMiddleware(out io.Writer) HttpMiddleware {
	return func(handler HandlerFunc) HandlerFunc {
		return func(srw *StreamResponseWriter, request *http.Request) {
			start := time.Now()
			handler.ServeHTTP(srw, request)
			fmt.Fprintf(out,
				"<- %s: %s %s %d (%s)\n",
				request.RemoteAddr,
				request.Method,
				request.URL,
				srw.statusCode,
				time.Since(start))
		}
	}
}

func NewAuthMiddleware(check AuthValidator) HttpMiddleware {
	return func(handler HandlerFunc) HandlerFunc {
		return func(srw *StreamResponseWriter, request *http.Request) {
			if !check(request) {
				srw.Header().Set("Access-Control-Allow-Origin", "*")
				srw.Header().Set("Access-Control-Allow-Methods", "GET")
				srw.WriteHeader(http.StatusUnauthorized)
				return
			}
			handler.ServeHTTP(srw, request)
		}
	}
}

func AdaptHandler(handler HandlerFunc, middleware ...HttpMiddleware) http.HandlerFunc {
	return func(writer http.ResponseWriter, request *http.Request) {
		srw := NewStatusResponseWriter(writer)
		wrapped := handler
		for _, m := range middleware {
			wrapped = m(wrapped)
		}
		wrapped.ServeHTTP(srw, request)
	}
}
