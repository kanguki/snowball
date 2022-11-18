package main

import "github.com/kanguki/snowball/network"

func main() {
	network.NewTcpJsonNode("30000")
	select {}
}
