package network

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"strconv"
	"sync"
	"time"
)

// TcpJsonNode is a kind of node in the network that communicates in plain text and json
type TcpJsonNode struct {
	//host of the node
	host string
	//port of the node
	port int
	//peers holds a list of other peer addresses
	peers map[string]bool
	//customHandler is used to plug in custom handler
	customHandler func(message []byte)
	//lock is used to save other pieces of data
	lock *sync.Mutex
	//timeout for a successful established connection, in second
	timeoutConnection int
	//maxRetriesJoinNetwork set limit for number of time trying to join the network
	maxRetriesJoinNetwork int
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

// only bootstrap server needs to have port before hand, other nodes will randomly get a port
func NewTcpJsonNode(host string, port int, timeoutConn, maxRetries int) *TcpJsonNode {
	node := &TcpJsonNode{
		host:                  host,
		port:                  port,
		peers:                 map[string]bool{},
		lock:                  &sync.Mutex{},
		timeoutConnection:     timeoutConn,
		maxRetriesJoinNetwork: maxRetries,
	}
	node.acceptMessages()
	return node
}

func (node *TcpJsonNode) MyAddress() string {
	return fmt.Sprintf("%s:%d", node.host, node.port)
}

func (node *TcpJsonNode) RegisterMsghandler(handler func(message []byte)) {
	node.customHandler = handler
}

// Join sends SELF_INTRODUCE and GET_PEERS to the bootstrapAddress
func (node *TcpJsonNode) Join(bootstrapAddress string) {
	join := func() error {
		err := node.sendPingMsg(bootstrapAddress, SELF_INTRODUCE, MsgPayload{Address: fmt.Sprintf("%s:%d", node.host, node.port)})
		if err != nil {
			return err
		}
		return node.sendPingMsg(bootstrapAddress, GET_PEER_LIST)
	}
	var err error
	left := node.maxRetriesJoinNetwork
	for left > 0 {
		err = join()
		if err == nil {
			return
		}
		left--
		log.Printf("%d times left: %s failed to join network. Error: %v\n", left, node.MyAddress(), err)
		time.Sleep(time.Second)
	}
	panic(fmt.Errorf("fail to join p2p network: %v", err))
}

// acceptMessages opens a tcp connection, listen for msgs and process
// if they are in correct format
func (node *TcpJsonNode) acceptMessages() {
	//Listen on a port, use random port if not specified
	addr := "localhost:"
	if node.host != "" {
		addr = node.host + ":"
	}
	if node.port != 0 {
		addr += fmt.Sprint(node.port)
	}
	listener, err := net.Listen("tcp", addr)
	times := 100
	for times > 0 && err != nil {
		times--
		if times == 0 {
			panic(err)
		}
		listener, err = net.Listen("tcp", addr)
	}
	host, port, err := net.SplitHostPort(listener.Addr().String())
	if err != nil {
		panic(err)
	}
	log.Printf("node starting on %s", port)
	portNum, _ := strconv.ParseInt(port, 10, 32) //TODO: handle error
	node.port = int(portNum)
	node.host = host

	// Listen for an incoming connection
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				panic(err)
			}
			// Handle connections in a new goroutine
			go func(conn net.Conn) {
				defer func() {
					// fmt.Println("Closing connection...")
					conn.Close()
				}()

				timeoutDuration := time.Duration(node.timeoutConnection) * time.Second
				bufReader := bufio.NewReader(conn)

				for {
					// Set a deadline for reading. Read operation will fail if no data
					// is received after deadline.
					conn.SetReadDeadline(time.Now().Add(timeoutDuration))

					// Read tokens delimited by newline
					bytes, err := bufReader.ReadBytes('\n')
					if err != nil && err != io.EOF {
						log.Printf("listener ReadBytes error: %v\n", err)
						return
					}

					if len(bytes) == 0 {
						return
					}

					//message pinging between nodes
					if bytes[0] == '0' {
						//parse payload if exist
						var payload MsgPayload
						err = json.Unmarshal(bytes[1:], &payload)
						if err != nil {
							log.Printf("parsing ping message errror: %v\n", err)
							return
						}
						// fmt.Printf("%+v\n", payload)
						go node.processmsg(payload)
					} else if node.customHandler != nil {
						go node.customHandler(bytes)
					}
				}
			}(conn)
		}
	}()
}

func (node *TcpJsonNode) processmsg(msg MsgPayload) {
	// fmt.Printf("receive %d from %v\n", msg.Type, msg.Address)
	switch msg.Type {

	case SELF_INTRODUCE:
		node.addPeerToNode(msg.Address)

	case GET_PEER_LIST:
		go node.sendPingMsg(msg.Address, PEERS_INTRODUCE, MsgPayload{AddressList: append(node.GetPeers(), fmt.Sprintf("%s:%d", node.host, node.port))})

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
			go node.sendPingMsg(addr, SELF_INTRODUCE, MsgPayload{})
		}
	}
}

// sendPingMsg forms a valid raw message and sends to the receiver
func (node *TcpJsonNode) sendPingMsg(receiver string, msgType MsgType,
	msg ...MsgPayload) error {
	message := MsgPayload{Type: msgType, Address: fmt.Sprintf("%s:%d", node.host, node.port)}
	if msg != nil {
		message = msg[0]
		message.Address = fmt.Sprintf("%s:%d", node.host, node.port)
		message.Type = msgType
	}
	msgBytes, err := json.Marshal(&message)
	if err != nil {
		log.Printf("sendPingMsg error: %v\n", err)
		return err
	}
	tosend := append([]byte{'0'}, msgBytes...)
	return node.SendMessage(receiver, tosend)
}

func (node *TcpJsonNode) SendMessage(address string, message []byte) error {
	conn, err := net.Dial("tcp", address)
	if err != nil {
		log.Printf("SendMessage %s error %v\n", message, err)
		return err
	}
	conn.Write(message)
	// fmt.Printf("sent message %s to %s\n", msgBytes, receiver)
	conn.Close()
	return nil
}

func (node *TcpJsonNode) addPeerToNode(address string) {
	if address == fmt.Sprintf("%s:%d", node.host, node.port) { //dont add myself
		return
	}
	node.lock.Lock()
	node.peers[address] = true
	node.lock.Unlock()
}

func (node *TcpJsonNode) GetPeers() []string {
	node.lock.Lock()
	defer node.lock.Unlock()
	peers := []string{}
	for addr := range node.peers {
		peers = append(peers, addr)
	}
	if len(peers) == 0 {
		return nil
	}
	return peers
}
