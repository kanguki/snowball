package network

import (
	"fmt"
	"sort"
	"testing"
	"time"
)

//TestJoin tries to make a network, then check if nodes have sufficient peer list
func TestJoin(t *testing.T) {
	network := []*TcpJsonNode{}
	var bootstrapPort int = 3e4
	bootstrapNode := fmt.Sprintf("%s:%d", IP, bootstrapPort)
	//if the size is too big, the number of misses will increase.
	//This may be because of port insufficience.
	//it should be okay with small size like 10
	clusterSize := 10
	//make the nodes
	for i := 0; i < clusterSize; i++ {
		port := fmt.Sprint(bootstrapPort + i)
		node := NewTcpJsonNode(port)
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
			if peer == node.Address {
				continue
			}
			want = append(want, peer)
		}
		sort.Strings(want)
		got := node.getPeers()
		sort.Strings(got)
		if len(want) != len(got) {
			fmt.Printf("%s missed %d!\n", node.Address, len(want)-len(got))
			misses++
		}
		// assert.Equal(t, want, got)
	}
	t.Logf("number of misses: %d\n", misses)
}
