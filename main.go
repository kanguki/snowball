package main

import "github.com/kanguki/snowball/network"


func main() {
	network.NewPlainTextNode("30000")
	select{}
}