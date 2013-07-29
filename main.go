package main

import (
    "log"
    "github.com/simon-weber/gomarkov"
    "github.com/simon-weber/gomegle"
    "net/http"
    "strings"
    "io/ioutil"
    "fmt"
    "os"
    "time"
    "math/rand"
)

func prepConversation() *botConversation{
    bot := markov.NewCustomChain(markov.WhitespaceTokenize, strings.ToLower)

    b, err := ioutil.ReadFile(fmt.Sprintf("%s/irccorpus", os.Getenv("OPENSHIFT_REPO_DIR")))
    if err != nil { fmt.Println(err) }

    for _, line := range strings.Split(string(b), "\n"){
        bot.Update(line)
    }

    return NewBotConversation(gomegle.NewSession(), bot, 2 * time.Second)
}

func main() {
    rand.Seed( time.Now().UTC().UnixNano())

    ip := os.Getenv("OPENSHIFT_DIY_IP")
    port := os.Getenv("OPENSHIFT_DIY_PORT")

    hub := NewHub(prepConversation())
    go hub.run()

    http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
        serveWs(w, r, hub)
    })

    addr := fmt.Sprintf("%s:%s", ip, port)
    log.Printf("Running on %s", addr)
    err := http.ListenAndServe(addr, nil)
    if err != nil {
        log.Fatal("ListenAndServe: ", err)
    }
}
