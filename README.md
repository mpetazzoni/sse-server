# A Server-Sent Events server for demonstration and testing

`sse-server` is a small server application that exposes a Server-Sent
Event stream at `/stream` that can be used for testing client
applications and SSE client libraries.

I've developed a few of those in my days
([`sseclient-py`](https://github.com/mpetazzoni/sseclient) in Python,
and [`sse.js`](https://github.com/mpetazzoni/sse.js) in JavaScript) and
always struggled to have simple, reliable event sources to connect to.

## Usage

The easiest is to build and run the Docker image:

```
$ docker build -t sse-server .
$ docker run -p 8080:8080 sse-server
```

Otherwise, you can run the Go program directly:

```
$ go build && ./sse-server
```

## Endpoints

### `/status`

The `/status` endpoint returns a JSON map of all connected clients and
their current event stream sequence position:

```
GET /status HTTP/1.1

HTTP/1.1 200 OK
Content-Length: 234
Content-Type: application/json
Date: Thu, 15 Feb 2024 22:15:36 GMT

{
    "192.168.65.1:35299": {
        "connectedAt": "2024-02-15T22:15:24.241547551Z",
        "lastEventId": 15,
        "remote": "192.168.65.1:35299"
    },
    "192.168.65.1:35305": {
        "connectedAt": "2024-02-15T22:15:34.537873958Z",
        "lastEventId": 5,
        "remote": "192.168.65.1:35305"
    }
}
```

### `/stream`

The `/stream` endpoint returns a Server-Sent Event stream
(`text/event-stream`). The stream always starts with an initialization
event:

```
id: hello
data: Hello, 192.168.65.1:35326!
```

It is then followed by a never-ending sequence of messages:

```
id: message-%d
data: {
data:   "time": CURRENT_UNIX_EPOCH,
data:   "random": 16_CHAR_RANDOM_STRING
data: }
```

The random string is always the same for a given message number.

This endpoint supports the `Last-Event-ID` header as defined by the SSE
specification, resuming the stream at the given sequence position.

```
GET /stream HTTP/1.1
Accept: */*
Accept-Encoding: gzip, deflate
Connection: keep-alive
Host: localhost:8080
Last-Event-ID: message-4
User-Agent: HTTPie/3.2.2



HTTP/1.1 200 OK
Cache-Control: no-cache
Connection: keep-alive
Content-Type: text/event-stream
Date: Thu, 15 Feb 2024 22:04:16 GMT
Transfer-Encoding: chunked

id: init
data: Hello!

id: message-4
data: {
data:   "time": 1708034656,
data:   "random": "IdyFCHZMVcGYbsTr"
data: }

...
```
