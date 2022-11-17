package network

//Message is a message that can be understood in the network.
//We use json for simplicity
type Message struct{
	Header MessageHeader `json:"header"`
	Body MessageBody `json:"body"`
}

type MessageBody struct {
	//Arbitrary json content
	Message map[string]interface{} `json:"message"`
}

type MessageHeader struct {
	//ID of the node sending message
	NodeID string `json:"ID"`
	//Type of the message
	Type MessageHeaderType `json:"type"`
}

type MessageHeaderType int64
const (
	//notify other nodes about the existence of the sender
	SELF_INTRODUCE MessageHeaderType = iota
	//notify other nodes about the existence of the address peers 
	PEERS_INTRODUCE
)