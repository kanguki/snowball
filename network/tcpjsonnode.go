package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// TcpJsonNode is a kind of node in the network that communicates in plain text and json
type TcpJsonNode struct {
	//Port of the node
	Port string
	//Address of the node
	Address string
	//listener receives incoming msg from other peers in the network
	listener net.Listener
	//ProcessQueue saves msgs received from network and parse to MsgPayload
	ProcessQueue chan MsgPayload
	//Peers holds a list of other peer addresses
	Peers map[string]bool
	//lock is used to save other pieces of data
	lock *sync.Mutex
}

type MsgPayload struct {
	//Type of the msg
	Type        MsgType  `json:"type"`    //required
	Address     string   `json:"address"` //required
	AddressList []string `json:"address_list,omitempty"`
}

type MsgType int64

const (
	//notify other nodes about the existence of the sender
	SELF_INTRODUCE MsgType = iota + 1
	//request to get the peer list of the receiver
	GET_PEER_LIST
	//notify other nodes about the existence of the address peers
	PEERS_INTRODUCE
)

const IP string = "127.0.0.1" //for simplicity, use localhost as my ip

// only bootstrap server needs to have port before hand, other nodes will randomly get a port
func NewTcpJsonNode(port string) *TcpJsonNode {
	node := &TcpJsonNode{
		Port:         port,
		Peers:        map[string]bool{},
		lock:         &sync.Mutex{},
		ProcessQueue: make(chan MsgPayload, 1),
	}
	node.acceptMessages()
	return node
}

// Join sends SELF_INTRODUCE and GET_PEERS to the bootstrapAddress
func (node *TcpJsonNode) Join(bootstrapAddress string) {
	node.sendmsg(bootstrapAddress, SELF_INTRODUCE, MsgPayload{Address: node.Address})
	node.sendmsg(bootstrapAddress, GET_PEER_LIST)
}

// AcceptMessages opens a tcp connection, listen for msgs and process if they
// are in correct format
func (node *TcpJsonNode) acceptMessages() {

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
	log.Printf("node starting on %s", port)
	node.listener = listener
	node.Port = port
	node.Address = fmt.Sprintf("%s:%s", IP, port)

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
					if err != nil && err != io.EOF {
						fmt.Println(err)
						return
					}

					//parse payload if exist
					var payload MsgPayload
					if len(bytes) == 0 {
						return
					}
					err = json.Unmarshal(bytes, &payload)
					if err != nil {
						fmt.Println(err)
						return
					}
					// fmt.Printf("%+v\n", payload)
					node.ProcessQueue <- payload
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
			// fmt.Printf("receive %d from %v\n", msg.Type, msg.Address)
			switch msg.Type {

			case SELF_INTRODUCE:
				node.addPeerToNode(msg.Address)

			case GET_PEER_LIST:
				go node.sendmsg(msg.Address, PEERS_INTRODUCE, MsgPayload{AddressList: append(node.getPeers(), node.Address)})

			case PEERS_INTRODUCE:
				//add the sender to the list too
				node.addPeerToNode(msg.Address)
				if msg.AddressList == nil {
					return
				}
				//for each peer in the list, save and send introduce msg to inform
				//them about my existence
				for _, addr := range msg.AddressList {
					node.addPeerToNode(addr)
					go node.sendmsg(addr, SELF_INTRODUCE, MsgPayload{})
				}

			}
		}
	}
}

// sendmsg forms a valid raw message and sends to the receiver
func (node *TcpJsonNode) sendmsg(receiver string, msgType MsgType, msg ...MsgPayload) {
	message := MsgPayload{Type: msgType, Address: node.Address}
	if msg != nil {
		message = msg[0]
		message.Address = node.Address
		message.Type = msgType
	}
	msgBytes, err := json.Marshal(&message)
	if err != nil {
		fmt.Println(err)
		return
	}
	conn, err := net.Dial("tcp", receiver)
	if err != nil {
		fmt.Println(err)
		return
	}
	conn.Write(msgBytes)
	// fmt.Printf("sent message %s to %s\n", msgBytes, receiver)
	conn.Close()
}

func (node *TcpJsonNode) addPeerToNode(address string) {
	if address == node.Address { //dont add myself
		return
	}
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
