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
	clusterSize := 30
	terms := 3
	choices := []string{"black", "blue", "red", "pink", "gray", "white", "orange", "yellow", "purple"}
	timeoutLoopQuerySampleK := 10 //in second, the bigger the more exact, should be increased if clusterSize is too big
	timeoutQuery1SampleK := 2     //in second, the bigger the more exact, should be increased if clusterSize is too big

	var bootstrapPort int = 4e4
	//make the network
	network := []net.Node{}
	bootstrapNode := net.NewTcpJsonNode(fmt.Sprint(bootstrapPort), 5, 5)

	//make the notification node
	notiNode := plugin.NewP2pNotification(bootstrapNode.MyAddress(), fmt.Sprint(bootstrapPort-1), 5, 5)

	//make the instances
	cluster := []*Consensus{}
	for i := 0; i < clusterSize-1; i++ {
		node := net.NewTcpJsonNode("", 5, 5)
		network = append(network, node)
		instance := NewConsensus(
			node,
			notiNode,
			SnowConfig{
				K:                   5,
				A:                   3,
				B:                   50,
				M:                   time.Duration(time.Duration(timeoutLoopQuerySampleK) * time.Second),
				TimeoutQuerySampleK: time.Duration(time.Duration(timeoutQuery1SampleK) * time.Second),
			},
		)
		cluster = append(cluster, instance)
	}
	for _, node := range network {
		node.Join(bootstrapNode.MyAddress())
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
