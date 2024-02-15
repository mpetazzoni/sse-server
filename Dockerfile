FROM golang:1.22 AS builder
WORKDIR /app
RUN useradd -u 10001 scratchuser
COPY go.mod *.go /app
RUN CGO_ENABLED=0 GOOS=linux go build

FROM scratch AS runtime
COPY --from=builder /etc/passwd /etc/passwd
COPY --from=builder /app/sse-server /sse-server
USER scratchuser
EXPOSE 8080
ENTRYPOINT ["/sse-server"]
