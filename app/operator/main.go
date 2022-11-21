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
	"github.com/rs/cors"
)

func main() {
	//parsing flag
	networkInterface := flag.String("nic", "localhost", "network interface that the node listens on")
	bootstrapAddress := flag.String("bootstraphost", "localhost:30000", "bootstrap address to join the p2p network")
	p2pPort := flag.Int("p2p_port", 29999, "p2p port this operator runs on")
	wsPort := flag.Int("ws_port", 3000, "ws port this operator runs on")
	flag.Parse()
	assertCorrectNetworkHostPort(*bootstrapAddress)

	//set up notification and ws server
	lock := &sync.Mutex{}
	msgQueue := make(chan snow.Message, 1)
	wsClients := make(map[*websocket.Conn]bool, 1)
	go func() {
		for change := range msgQueue {
			for client := range wsClients {
				client.WriteJSON(change)
			}
		}
	}()
	//notification
	notiNode := plugin.NewP2pNotificationServer(*bootstrapAddress, *networkInterface, *p2pPort, 5, 10)
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
		msgQueue <- msg
	})

	//http calls
	//get all peer list
	mux := http.NewServeMux()
	mux.HandleFunc("/nodes", func(w http.ResponseWriter, r *http.Request) {
		peers := notiNode.GetPeers()
		bytes, err := json.Marshal(&peers)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Header().Set("Content-Type", "application/json")
		w.Write(bytes)
	})
	//set color for a certain term
	mux.HandleFunc("/set", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			w.WriteHeader(http.StatusBadRequest)
			w.Write([]byte("bad request. only accept POST"))
			return
		}
		var message struct {
			Term   int               `json:"term"`
			Choice map[string]string `json:"choice"`
		}
		err := json.NewDecoder(r.Body).Decode(&message)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if message.Choice == nil {
			w.WriteHeader(http.StatusNoContent)
			w.Write([]byte("empty choice"))
			return
		}
		for address, choice := range message.Choice {
			if choice == "" {
				continue
			}
			_, _, err := net.SplitHostPort(address)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			bytes, err := json.Marshal(&snow.Message{
				Type: snow.GENERATE,
				Data: snow.Choice{Term: message.Term, Color: choice},
			})
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			go notiNode.SendMessage(address, append([]byte{snow.FIRST_BIT}, bytes...))
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	})
	//websocket
	mux.HandleFunc("/ws", func(w http.ResponseWriter, r *http.Request) {
		upgrader := websocket.Upgrader{
			ReadBufferSize:  1024,
			WriteBufferSize: 2048,
			CheckOrigin: func(r *http.Request) bool {
				return true
			},
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
				// log.Printf("ws read message error: %v\n", err)
				lock.Lock()
				delete(wsClients, ws)
				lock.Unlock()
				break
			}
		}
	})

	handler := cors.AllowAll().Handler(mux)
	log.Printf("Serving ws on port %d\n", *wsPort)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *wsPort), handler))
}

func assertCorrectNetworkHostPort(addr string) {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		log.Fatalf("%v is not a valid host. example: localhost:3000", addr)
	}
}
