package main

import (
	"flag"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/kanguki/snowball/network"
	"github.com/kanguki/snowball/plugin"
	"github.com/kanguki/snowball/snow"
)

func main() {
	rand.Seed(time.Now().UnixMilli())
	//parsing flag
	bootstrapAddress := flag.String("bootstraphost", "localhost:30000", "bootstrap address to join the p2p network")
	p2pPort := flag.String("p2p_port", "", "port for p2p node in the network")
	notiAddress := flag.String("noti_addr", "127.0.0.1:29999", "address that the notification node is running on")
	timeoutPerLoop := flag.Int("timeout_loop", 30, "timeout in second for each snow ball loop")
	timeoutPerQuery := flag.Int("timeout_query", 2, "timeout in second for each sampkeK query round")
	sampleK := flag.Int("K", 5, "number of nodes per sample")
	thresholdA := flag.Int("A", 3, "number of votes that can be considered an aggreement")
	roundB := flag.Int("B", 100, "number of rounds to query sampleK")

	flag.Parse()
	assertCorrectNetworkHostPort(*bootstrapAddress)
	assertCorrectNetworkHostPort(*notiAddress)

	notiClient := plugin.NewP2pNotificationClient(*notiAddress)
	node := network.NewTcpJsonNode(*p2pPort, 5, 5)
	config := snow.SnowConfig{
		K:                   *sampleK,
		A:                   *thresholdA,
		B:                   *roundB,
		M:                   time.Duration(time.Duration(*timeoutPerLoop) * time.Second),
		TimeoutQuerySampleK: time.Duration(time.Duration(*timeoutPerQuery) * time.Second),
	}
	node.Join(*bootstrapAddress)
	snow.NewConsensus(node, notiClient, config)
	select {}
}

func assertCorrectNetworkHostPort(addr string) {
	_, _, err := net.SplitHostPort(addr)
	if err != nil {
		log.Fatalf("%v is not a valid host. example: localhost:3000", addr)
	}
}
