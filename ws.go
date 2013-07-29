package main

import (
    "github.com/garyburd/go-websocket/websocket"
    "log"
    "net/http"
    "time"
    "strings"
)

const (
    allowedTimeouts = 5

    // Time allowed to write a message to the client.
    writeWait = 10 * time.Second

    // Time allowed to read the next message from the client.
    readWait = 10 * time.Second

    // Send pings to client with this period. Must be less than readWait.
    pingPeriod = (readWait * 9) / 10

    // Maximum message size allowed from client.
    maxMessageSize = 512
)

// connection is an middleman between the websocket connection and the hub.
type connection struct {
    // The websocket connection.
    ws *websocket.Conn

    // Buffered channel of outbound messages.
    send chan []byte

    hub *hub
}

// readPump pumps messages from the websocket connection to the hub.
func (c *connection) readPump() {
    timeouts := 0

    defer func() {
        c.hub.unregister <- c
        c.ws.Close()
    }()

    c.ws.SetReadLimit(maxMessageSize)
    c.ws.SetReadDeadline(time.Now().Add(readWait))

    for {
        op, _, err := c.ws.NextReader()
        if err != nil {
            if strings.Contains(err.Error(), "timeout"){
                log.Println("connection time out: %s", c)
                timeouts++
                if timeouts > allowedTimeouts{
                    break
                } else {
                    continue
                }
            } else{
                log.Printf("couldn't open readPump reader: %s\n", err)
                break
            }
        }
        switch op {
        case websocket.OpPong:
            c.ws.SetReadDeadline(time.Now().Add(readWait))
        case websocket.OpText:
            //message, err := ioutil.ReadAll(r)
            //if err != nil {
            //    break
            //}
            //h.broadcast <- message
        }
    }
}

// write writes a message with the given opCode and payload.
func (c *connection) write(opCode int, payload []byte) error {
    c.ws.SetWriteDeadline(time.Now().Add(writeWait))
    return c.ws.WriteMessage(opCode, payload)
}

// writePump pumps messages from the hub to the websocket connection.
func (c *connection) writePump() {
    ticker := time.NewTicker(pingPeriod)
    defer func() {
        ticker.Stop()
        c.ws.Close()
    }()
    for {
        select {
        case message, ok := <-c.send:
            if !ok {
                log.Println("writePump: send closed")
                c.write(websocket.OpClose, []byte{})
                return
            }
            if err := c.write(websocket.OpText, message); err != nil {
                log.Printf("writePump: error writing text (%s)\n", err)
                return
            }
        case <-ticker.C:
            if err := c.write(websocket.OpPing, []byte{}); err != nil {
                log.Printf("writePump: error writing ping (%s)\n", err)
                return
            }
        }
    }
}

// serverWs handles webocket requests from the client.
func serveWs(w http.ResponseWriter, r *http.Request, hub *hub) {
    if r.Method != "GET" {
        http.Error(w, "Method not allowed", 405)
        return
    }

    //if r.Header.Get("Origin") != "http://"+r.Host {
    //    http.Error(w, "Origin not allowed", 403)
    //    return
    //}

    ws, err := websocket.Upgrade(w, r.Header, nil, 1024, 1024)
    if _, ok := err.(websocket.HandshakeError); ok {
        http.Error(w, "Not a websocket handshake", 400)
        return
    } else if err != nil {
        log.Println(err)
        return
    }
    c := &connection{send: make(chan []byte, 256), ws: ws, hub: hub}
    hub.register <- c
    go c.writePump()
    c.readPump()
}
