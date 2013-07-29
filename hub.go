package main

import (
    "time"
    "github.com/simon-weber/gomegle"
    "log"
    "encoding/json"
)

// hub maintains a botconversation, the set of active connections
// and broadcasts messages to the connections.
type hub struct {
    // Registered connections.
    connections map[*connection]bool

    // Register requests from the connections.
    register chan *connection

    // Unregister requests from connections.
    unregister chan *connection

    conversation *botConversation
}

func NewHub(conversation *botConversation) *hub{
    return &hub{
        register:    make(chan *connection),
        unregister:  make(chan *connection),
        connections: make(map[*connection]bool),
        conversation: conversation,
    }
}


func (h *hub) run() {
    // events that go to our connections
    cEvents := make(chan *gomegle.Event, 128)

    // event handling/transitions:
    //  new connection + not inConversation: start conversation
    //
    //  conversation over + have clients:    reconnect
    //  conversation over + no clients:      inConversation = false
    //
    //  first timeout:                       prompt stranger with a message
    //  second timeout:                      disconnect (will trigger conversation over)

    inConversation := false
    conversationOver := make(chan bool)

    strangerTimeout := 15 * time.Second
    willPromptOnTimeout := true
    maxRecv := 200

    // we'll start this timer once we enter a conversation
    strangerTimer := time.NewTimer(strangerTimeout)
    strangerTimer.Stop()

    // shared handling logic
    tStartConversation := func(){
        log.Println("starting conversation")
        inConversation = true
        willPromptOnTimeout = true

        go func(){
            h.conversation.converse(cEvents, maxRecv)
            conversationOver <- true
        }()
    }


    for {
        select {
        case c := <-h.register:
            h.connections[c] = true

            if !inConversation{
                log.Println("starting conversation for new client")
                tStartConversation()
            } else {
                event := gomegle.Event{Kind: "info", Value: "<joined in the middle of a conversation>"}
                msg, err := json.Marshal(event)

                if err != nil{
                    log.Printf("unable to Marshal event: %s\n", event)
                } else{
                    c.send <- msg
                }
            }

        case c := <-h.unregister:
            delete(h.connections, c)
            close(c.send)

        case event := <-cEvents:
            log.Println("relevant event:", event)
            switch event.Kind{
            case "connected":
                strangerTimer.Reset(strangerTimeout)
            case "weDisconnected", "strangerDisconnected", "weMessage":
                // do nothing, currently
            case "gotMessage":
                strangerTimer.Reset(strangerTimeout)
                willPromptOnTimeout = true
            }

            msg, err := json.Marshal(event)

            if err != nil{
                log.Println("unable to Marshal event")
            } else{
                for c, _ := range h.connections{
                    c.send <- msg
                }
            }

        case <-conversationOver:
            strangerTimer.Stop()
            if len(h.connections) > 0{
                log.Println("have clients; reconnecting")
                tStartConversation()
            } else {
                log.Println("no clients; not reconnecting")
                inConversation = false
            }

        case <-strangerTimer.C:
            if willPromptOnTimeout{
                log.Println("timeout 1: prompting")
                h.conversation.say(1,3) // want a short message, like a greeting
                willPromptOnTimeout = false
                strangerTimer.Reset(strangerTimeout)
            } else{
                log.Println("timeout 2: disconnecting")
                h.conversation.requestStop()
            }
        }

    }
}
