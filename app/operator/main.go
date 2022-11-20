// operator knows about state of the consensus network by running a notification daemon,
// and serve http & ws server for user queries
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/kanguki/snowball/plugin"
	"github.com/kanguki/snowball/snow"
)

func main() {
	//parsing flag
	bootstrapAddress := flag.String("bootstraphost", "localhost:30000", "bootstrap address to join the p2p network")
	_, _, err := net.SplitHostPort(*bootstrapAddress)
	if err != nil {
		log.Fatalf("bootstraphost is not a valid host. example: localhost:3000")
	}
	p2pPort := flag.Int("p2p_port", 29999, "p2p port this operator runs on")
	wsPort := flag.Int("ws_port", 3000, "ws port this operator runs on")
	flag.Parse()

	//set up notification and ws server
	lock := &sync.Mutex{}
	msgQueue := make(chan snow.Choice, 1)
	wsClients := make(map[*websocket.Conn]bool, 1)
	go func() {
		for change := range msgQueue {
			for client := range wsClients {
				client.WriteJSON(change)
			}
		}
	}()
	//notification
	notiNode := plugin.NewP2pNotification(*bootstrapAddress, fmt.Sprint(*p2pPort), 5, 10)
	notiNode.RegisterMsghandler(func(msgBytes []byte) {
		if msgBytes[0] != plugin.FIRST_BIT {
			return
		}
		//notiNode is in the consensus network so it understands the network Message format
		var msg snow.Message
		err := json.Unmarshal(msgBytes[1:], &msg)
		if err != nil {
			log.Printf("error parsing notification message: %v\n", err)
			return
		}
		msgQueue <- msg.Data
	})

	//websocket
	http.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 2048,
		}
		ws, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Printf("websocket upgrade error: %v\n", err)
		}
		defer ws.Close()

		lock.Lock()
		wsClients[ws] = true
		lock.Unlock()

		ws.SetPongHandler(func(string) error {
			ws.SetReadDeadline(time.Now().Add(5 * time.Second))
			return nil
		})

		for {
			_, _, err := ws.ReadMessage()
			if err != nil {
				lock.Lock()
				delete(wsClients, ws)
				lock.Unlock()
				break
			}
		}
	})
	log.Printf("Serving ws on port %d\n", *wsPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *wsPort), nil))
}
