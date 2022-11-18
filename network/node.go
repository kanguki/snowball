// package network introduces functions of nodes in the p2p network
package network

// Node is a node in the network
type Node interface {
	//Join connects to one address and get the address lists that address is holding,
	//then for each of the address in the list, ping them so they know a node has just joined.
	Join(bootstrapAddress string)
	//Connect connects to peerID so they know a node has just joined.
	Connect(peerID string) error
	//SendMessage sends message to peerID
	SendMessage(peerID string, message []byte) error
	//AcceptMessages accepts incoming requests from other peers broadcasting about
	//themselves and add to the address list
	AcceptMessages()
}
