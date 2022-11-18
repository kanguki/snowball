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

// TcpJsonNode is a kind of node in the network that communicates in plain text and json
// a valid raw tcp msg this node accept is:
// 0{"json":"canBeEmpty"} where 0 is the msg type and json can be empty
type TcpJsonNode struct {
	//Port on localhost that the node is running
	Port string
	//listener receives incoming msg from other peers in the network
	listener net.Listener
	//ProcessQueue saves msgs received from network and parse to msg
	ProcessQueue chan msg
	//Peers holds a list of other peer addresses
	Peers map[string]bool
	//lock is used to save other pieces of data
	lock *sync.Mutex
}

// msg is a msg that can be understood in the network.
// We use json for simplicity
type msg struct {
	Header  msgHeader
	Payload msgPayload
}

type msgPayload struct {
	AddressList []string `json:"address_list,ommitempty"`
}

type msgHeader struct {
	//Type of the msg
	Type msgType
	//Address of the sender, sender doesnt need to send, it's retrieved from connection
	Address string
}

type msgType int64

const (
	//notify other nodes about the existence of the sender
	SELF_INTRODUCE msgType = iota
	//request to get the peer list of the receiver
	GET_PEER_LIST
	//notify other nodes about the existence of the address peers
	PEERS_INTRODUCE
)

// only bootstrap server needs to have port before hand, other nodes will randomly get a port
func NewTcpJsonNode(port string) (*TcpJsonNode, error) {
	node := &TcpJsonNode{
		Port:         port,
		Peers:        map[string]bool{},
		lock:         &sync.Mutex{},
		ProcessQueue: make(chan msg, 1),
	}
	node.AcceptMessages()
	log.Printf("node starting on port %s", node.Port)
	return node, nil
}

// AcceptMessages opens a tcp connection, listen for msgs and process if they
// are in correct format
func (node *TcpJsonNode) AcceptMessages() {

	//Listen on a port, use random port if not specified
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

					//parse payload if exist
					var payload msgPayload
					if len(bytes) > 1 {
						err = json.Unmarshal(bytes[1:], &payload)
						if err != nil {
							return
						}
					}
					msg := msg{
						Header: msgHeader{
							Address: conn.RemoteAddr().String(),
							Type:    msgType(bytes[0] - '0'),
						},
						Payload: payload,
					}
					fmt.Printf("%+v\n", msg)
					node.ProcessQueue <- msg
				}
			}(conn)
		}
	}()
	go node.processmsgs()
}

func (node *TcpJsonNode) processmsgs() {
	for {
		select {
		case msg := <-node.ProcessQueue:
			fmt.Printf("receive %d from %v\n", msg.Header.Type, msg.Header.Address)
			switch msg.Header.Type {

			case SELF_INTRODUCE:
				node.addPeerToNode(msg.Header.Address)

			case GET_PEER_LIST:
				go node.sendmsg(msg.Header.Address, PEERS_INTRODUCE, msgPayload{AddressList: node.getPeers()})

			case PEERS_INTRODUCE:
				//add the sender to the list too
				node.addPeerToNode(msg.Header.Address)
				if msg.Payload.AddressList == nil {
					return
				}
				//for each peer in the list, save and send introduce msg to inform
				//them about my existence
				for _, addr := range msg.Payload.AddressList {
					node.addPeerToNode(addr)
					go node.sendmsg(addr, SELF_INTRODUCE)
				}

			}
		}
	}
}

// sendmsg forms a valid raw message and sends to the receiver
func (node *TcpJsonNode) sendmsg(receiver string, msgType msgType, msg ...msgPayload) {
	conn, err := net.Dial("tcp", receiver)
	if err != nil {
		fmt.Println(err)
		return
	}
	msgBytes, err := json.Marshal(&msg)
	if err != nil {
		return
	}
	conn.Write(append([]byte{byte(msgType)}, msgBytes...))
	conn.Close()
}

func (node *TcpJsonNode) addPeerToNode(address string) {
	node.lock.Lock()
	node.Peers[address] = true
	node.lock.Unlock()
}

func (node *TcpJsonNode) getPeers() []string {
	node.lock.Lock()
	defer node.lock.Unlock()
	peers := []string{}
	for addr := range node.Peers {
		peers = append(peers, addr)
	}
	if len(peers) == 0 {
		return nil
	}
	return peers
}
