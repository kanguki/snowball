package plugin

import (
	"encoding/json"
	"log"

	"github.com/kanguki/snowball/network"
)

// Notification is used to notify
type Notification interface {
	//NotifyChange works whenever the value is switched
	NotifyChange(node network.Node, what interface{}) error
	//Node is the daemon node sitting receiving messages
	network.Node
}

type P2pNotification struct {
	network.Node
}

func NewP2pNotification(bootstrapAddress, port string, timeoutConn, maxRetries int) *P2pNotification {
	node := network.NewTcpJsonNode(port, timeoutConn, maxRetries)
	node.Join(bootstrapAddress)
	return &P2pNotification{
		Node: node,
	}
}

func (p *P2pNotification) NotifyChange(node network.Node, what interface{}) error {
	msgBytes, err := json.Marshal(&what)
	if err != nil {
		log.Printf("NotifyChange error: %v\n", err)
		return err
	}
	err = node.SendMessage(p.MyAddress(), msgBytes)
	if err != nil {
		log.Printf("sendDecision SendMessage error: %v\n", err)
	}
	return err
}
