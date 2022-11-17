package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"sync"
	"time"
)

// PlainTextNode is a kind of node in the network that communicates in plain text
type PlainTextNode struct {
	//ID of the node
	ID string
	//Port on localhost that the node is running
	Port string
	//listener receives incoming message from other peers in the network
	listener net.Listener
	//ProcessQueue saves messages received from network and parse to Message
	ProcessQueue chan Message
	//Peers holds a map of peerID and its corresponding address
	Peers map[string]string
	//lock is used to save other pieces of data
	lock *sync.Mutex
}

// only bootstrap server needs to have port before hand, other nodes will randomly get a port
func NewPlainTextNode(port string) (*PlainTextNode, error) {
	node := &PlainTextNode{
		ID:           fmt.Sprint(port),
		Port:         port,
		Peers:        map[string]string{},
		lock:         &sync.Mutex{},
		ProcessQueue: make(chan Message, 1),
	}
	node.AcceptMessages()
	log.Printf("node starting on port %s", node.Port)
	return node, nil
}

func (node *PlainTextNode) AcceptMessages() {
	//Listen on a port
	addr := "localhost:"
	if node.Port != "" {
		addr += node.Port
	}
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		panic(err)
	}
	_, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		panic(err)
	}
	node.listener = listener
	node.Port = port
	// Listen for an incoming connection
	go func() {
		for {
			conn, err := node.listener.Accept()
			if err != nil {
				panic(err)
			}
			// Handle connections in a new goroutine
			go func(conn net.Conn) {
				defer func() {
					// fmt.Println("Closing connection...")
					conn.Close()
				}()

				timeoutDuration := 5 * time.Second
				bufReader := bufio.NewReader(conn)

				for {
					// Set a deadline for reading. Read operation will fail if no data
					// is received after deadline.
					conn.SetReadDeadline(time.Now().Add(timeoutDuration))

					// Read tokens delimited by newline
					bytes, err := bufReader.ReadBytes('\n')
					if err != nil {
						return
					}
					var message Message
					err = json.Unmarshal(bytes, &message)
					if err == nil {
						fmt.Printf("%+v\n", message)
						node.ProcessQueue <- message
					}
				}
			}(conn)
		}
	}()
	go node.processMessages()
}

func (node *PlainTextNode) processMessages() {
	for {
		select {
		case message := <-node.ProcessQueue:
			switch message.Header.Type {
			case SELF_INTRODUCE:
				go func() {
					id := message.Header.NodeID
					if id == "" {
						return
					}
					fmt.Printf("receive SELF_INTRODUCE from %v\n", id)
					addr := message.Body.Message["address"]
					if addr == nil {
						return
					}
					host, port, err := net.SplitHostPort(fmt.Sprint(addr))
					if err != nil {
						return
					}
					//broadcast the address list to the sender if it requires
					iNeedAddressList := message.Body.Message["iNeedAddressList"]
					if iNeedAddressList != nil && fmt.Sprint(iNeedAddressList) == "1" {
						conn, err := net.Dial("tcp", fmt.Sprint(addr))
						if err != nil {
							fmt.Println(err)
							return
						}
						message := Message{Header: MessageHeader{NodeID: node.ID, Type: PEERS_INTRODUCE},
							Body: MessageBody{Message: map[string]interface{}{"addr_list": node.Peers}}}
						messageBytes, err := json.Marshal(&message)
						if err != nil {
							return
						}
						conn.Write(messageBytes)
						conn.Close()
					}
					//add peer to address list
					node.addPeerToNode(id, fmt.Sprintf("%s:%s", host, port))
				}()
			case PEERS_INTRODUCE:
				go func() {
					id := message.Header.NodeID
					if id == "" {
						return
					}
					fmt.Printf("receive PEERS_INTRODUCE from %v\n", id)
					addrList := message.Body.Message["addr_list"] //addrList is a map of peerID_address
					if addrList == nil {
						return
					}
					addrMap, ok := addrList.(map[string]interface{})
					if !ok {
						return
					}
					for id, addr := range addrMap {
						if addr != nil {
							_, _, err := net.SplitHostPort(fmt.Sprint(addr))
							if err != nil {
								return
							}
						}
						node.addPeerToNode(id, fmt.Sprint(addr))
					}
					fmt.Println(node.Peers)
				}()
			}
		}
	}
}

func (node *PlainTextNode) addPeerToNode(peerId, address string) {
	node.lock.Lock()
	node.Peers[peerId] = address
	node.lock.Unlock()
}
