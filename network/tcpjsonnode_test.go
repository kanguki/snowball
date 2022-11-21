package network

import (
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// TestJoin tries to make a network, then check if nodes have sufficient peer list
func TestJoin(t *testing.T) {
	const IP string = "127.0.0.1"
	network := []Node{}
	var bootstrapPort int = 3e4
	bootstrapNode := fmt.Sprintf("%s:%d", IP, bootstrapPort)
	//if the size is too big, the number of misses will increase.
	//This may be because of port insufficience.
	//it should be okay with small size like 10
	clusterSize := 10
	//make the nodes
	for i := 0; i < clusterSize; i++ {
		node := NewTcpJsonNode(IP, bootstrapPort+i, 5, 5)
		network = append(network, node)
	}
	for _, node := range network {
		node.Join(bootstrapNode)
	}
	time.Sleep(time.Second) //time for nodes to talk to each other
	misses := 0
	for _, node := range network {
		want := []string{}
		for i := 0; i < clusterSize; i++ {
			peer := fmt.Sprintf("%s:%d", IP, bootstrapPort+i)
			if peer == node.MyAddress() {
				continue
			}
			want = append(want, peer)
		}
		sort.Strings(want)
		got := node.GetPeers()
		sort.Strings(got)
		if len(want) != len(got) {
			fmt.Printf("%s missed %d!\n", node.MyAddress(), len(want)-len(got))
			misses++
		}
		assert.Equal(t, want, got)
	}
	t.Logf("number of misses: %d\n", misses)
}
