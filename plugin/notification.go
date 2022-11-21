package plugin

import (
	"encoding/json"
	"log"
	"net"
	"time"

	"github.com/kanguki/snowball/network"
)

// FIRST_BIT bit is used for fast filtering as there are other types of messages in the network too.
const FIRST_BIT byte = '2'

type P2pNotificationServer struct {
	network.Node
}

func NewP2pNotificationServer(bootstrapAddress, host string, port int, timeoutConn, maxRetries int) *P2pNotificationServer {
	node := network.NewTcpJsonNode(host, port, timeoutConn, maxRetries)
	node.Join(bootstrapAddress)
	return &P2pNotificationServer{
		Node: node,
	}
}

type NotificationClient interface {
	//NotifyChange works whenever the value is switched
	NotifyChange(node network.Node, what interface{}) error
	//MyAddress is the address of the node in the p2p network
	MyAddress() string
}

type P2pNotificationClient struct {
	address string
}

func NewP2pNotificationClient(serverAddress string) *P2pNotificationClient {
	times := 10
	conn, err := net.Dial("tcp", serverAddress)
	for times > 0 && err != nil {
		times--
		time.Sleep(time.Second)
		if times == 0 {
			log.Printf("timeout connecting to notification server. error: %v\n", err)
			return &P2pNotificationClient{address: serverAddress}
		}
		// log.Printf("%d times left: error connecting to notification server: %v\nuse provided address %s then", times, err, serverAddress)
		conn, err = net.Dial("tcp", serverAddress)
	}
	conn.Close()
	return &P2pNotificationClient{address: conn.RemoteAddr().String()}
}

func (p *P2pNotificationClient) MyAddress() string {
	return p.address
}

func (p *P2pNotificationClient) NotifyChange(node network.Node, what interface{}) error {
	msgBytes, err := json.Marshal(&what)
	if err != nil {
		log.Printf("NotifyChange error: %v\n", err)
		return err
	}
	tosend := append([]byte{FIRST_BIT}, msgBytes...)
	err = node.SendMessage(p.MyAddress(), tosend)
	if err != nil {
		log.Printf("sendDecision SendMessage error: %v\n", err)
	}
	return err
}
