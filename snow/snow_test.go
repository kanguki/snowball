package snow

import (
	"fmt"
	"sync"
	"testing"
	"time"

	net "github.com/kanguki/snowball/network"
	"github.com/kanguki/snowball/plugin"
)

func TestAcceptChoice(t *testing.T) {
	//increase these params to stress the test :)
	clusterSize := 50
	terms := 5
	choices := []string{"black", "blue", "red", "pink", "gray", "white", "orange", "yellow", "purple"}
	timeoutQuerySampleK := 10 //second, should be increased if clusterSize is too big
	timeoutQueryEachPeer := 1 //second, should be increased if clusterSize is too big

	var bootstrapPort int = 4e4
	//make the network
	network := []net.Node{}

	//make the notification node
	notiNode := net.NewTcpJsonNode(fmt.Sprint(bootstrapPort - 1))
	network = append(network, notiNode)

	//make the instances
	bootstrapNode := fmt.Sprintf("localhost:%d", bootstrapPort)
	cluster := []*Consensus{}
	for i := 0; i < clusterSize; i++ {
		port := fmt.Sprint(bootstrapPort + i)
		node := net.NewTcpJsonNode(port)
		network = append(network, node)
		instance := NewConsensus(
			node,
			&plugin.P2pNotification{Address: network[0].MyAddress()},
			SnowConfig{
				K:             5,
				A:             3,
				B:             50,
				M:             time.Duration(time.Duration(timeoutQuerySampleK) * time.Second),
				Timeout1Query: time.Duration(time.Duration(timeoutQueryEachPeer) * time.Second),
			},
		)
		cluster = append(cluster, instance)
	}
	for _, node := range network {
		node.Join(bootstrapNode)
	}
	time.Sleep(time.Second)

	var wg sync.WaitGroup
	for term := 1; term <= terms; term++ {
		for idx := range cluster {
			wg.Add(1)
			go func(node, term int) {
				defer wg.Done()
				cluster[node].AcceptChoice(Choice{Term: term, Color: choices[node%len(choices)]})
			}(idx, term)
		}
	}
	wg.Wait()
	for term := 1; term <= terms; term++ {
		color := cluster[0].Values[term]
		for _, instance := range cluster {
			// t.Logf("on term %v node %v chose %v\n", term, instance.MyAddress(), instance.Values[term])
			if instance.Values[term] != color {
				t.Fatalf("consensus failed for term %v at node %v\n", term, instance.Node.MyAddress())
			}
		}
		t.Logf("consensus for term %d: %s\n", term, color)
	}
}
