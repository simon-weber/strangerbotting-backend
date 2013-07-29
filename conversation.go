package main

import (
    "github.com/simon-weber/gomarkov"
    "github.com/simon-weber/gomegle"
    "log"
    "time"
)

type botConversation struct {
    omegle *gomegle.Session
    bot *markov.Chain
    minResponseDelta time.Duration  // don't respond faster than this
}

func NewBotConversation(
    omegle *gomegle.Session,
    bot *markov.Chain,
    minResponseDelta time.Duration,
) *botConversation{
    return &botConversation{
        omegle: omegle,
        bot: bot,
        minResponseDelta: minResponseDelta,
    }
}

// say something random to the stranger
func (bc *botConversation) say(minLen int, maxLen int){
    resp, err := bc.bot.Respond("", minLen, maxLen)
    if err != nil{
        log.Printf("bc.say: %s", err)
    }
    bc.omegle.Say(resp)
}

// a disconnect event will eventually be sent on the normal output channel
func (bc *botConversation) requestStop(){
    bc.omegle.Disconnect()
}

// send relevant events on events chan
// disconnect after maxRecv messages have been received
func (bc *botConversation) converse(events chan *gomegle.Event, maxRecv int){
    // only relevant events go on client-provided channel
    allEvents := make(chan *gomegle.Event, 64)
    learnBuffer := make([]string, 0)

    go func(){
        lastMessageTime := time.Now()

        for event := range allEvents {
            switch event.Kind{
            case "connected", "weDisconnected", "strangerDisconnected", "weMessage":
                events <- event
            case "gotMessage":
                events <- event
                learnBuffer = append(learnBuffer, event.Value)

                recvTime := time.Now()

                if recvTime.Sub(lastMessageTime) > bc.minResponseDelta {
                    resp, err := bc.bot.Respond(event.Value, 2, 10)
                    if err != nil{
                        log.Printf("converse Respond error: %s", err)
                    }
                    bc.omegle.Say(resp)
                }

                lastMessageTime = recvTime

                if len(learnBuffer) > maxRecv{
                    bc.omegle.Disconnect()
                }
            }
        }
    }()

    if err := bc.omegle.Connect(allEvents); err != nil {
        log.Printf("converse Connect error: %s" ,err)
    }

    for _, msg := range learnBuffer{
        bc.bot.Update(msg)
    }

    log.Printf("learned from %d messages\n", len(learnBuffer))
}
