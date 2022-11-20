package snow

import (
	"context"
	"encoding/json"
	"log"
	"math/rand"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/kanguki/snowball/network"
	"github.com/kanguki/snowball/plugin"
)

// for easy visualization, I'll use color string for choice, but it can be any interface
type Choice struct {
	Term  int
	Color string
}

// Consensus is an implementation of Snowball consensus in a p2p network
type Consensus struct {
	noti         plugin.Notification //notification that knows about state of Values
	network.Node                     //node that can communicate in a p2p network
	SnowConfig                       //Config to perform a snow consensus

	Values               map[int]string         //Values of Consensus for each round
	valuesLock           *sync.Mutex            //protect Values
	queryResultQueue     map[string]chan Choice //list of channels holding QUERY_RESULT messages
	queryResultQueueLock *sync.Mutex            //protect queryResultQueue
}

func NewConsensus(p2pNode network.Node, pluginNoti plugin.Notification,
	config SnowConfig) *Consensus {
	ret := &Consensus{
		noti:                 pluginNoti,
		Node:                 p2pNode,
		Values:               map[int]string{},
		valuesLock:           &sync.Mutex{},
		queryResultQueue:     map[string]chan Choice{},
		queryResultQueueLock: &sync.Mutex{},
		SnowConfig:           config,
	}
	ret.RegisterMsghandler(ret.onMessage)
	return ret
}

type SnowConfig struct {
	K                   int           //sample K of each round of query. K < number_of_peers
	A                   int           //number of threshold that can be considered a majority. A < K
	B                   int           //number of rounds of successive aggreement of sample K
	M                   time.Duration //timeout to decide the choice
	TimeoutQuerySampleK time.Duration
}

// FIRST_BIT bit is used for fast filtering as there are other types of messages in the network too.
const FIRST_BIT byte = '1'

// Message is format of message amongst consensus nodes in the network
type Message struct {
	ID      string      `json:"id,omitempty"`
	Type    MessageType `json:"type"`
	Address string      `json:"address"` //address of the sender
	Data    Choice      `json:"data"`
}

type MessageType int

const (
	GENERATE MessageType = iota + 1
	QUERY
	QUERY_RESULT
	CHANGE
)

// OnQuery decides its value upon receiving QUERY messages from other peers
// its logics follow the white paper
func (i *Consensus) OnQuery(msg Message) {
	term := msg.Data.Term
	i.valuesLock.Lock()
	mine, ok := i.Values[term]
	i.valuesLock.Unlock()
	if ok {
		i.sendDecision(msg.Address, msg.ID, Choice{Term: term, Color: mine})
		return
	}
	i.switchValue(msg.Data)
	i.sendDecision(msg.Address, msg.ID, msg.Data)
	i.AcceptChoice(msg.Data)
}

// sendDecision sends my decision upon receiving QUERY message
func (i *Consensus) sendDecision(peerAddress string, id string, choice Choice) {
	msg := Message{
		ID:      id,
		Address: i.MyAddress(),
		Type:    QUERY_RESULT,
		Data:    choice,
	}
	msgBytes, err := json.Marshal(&msg)
	if err != nil {
		log.Printf("sendDecision error: %v\n", err)
		return
	}
	tosend := append([]byte{FIRST_BIT}, msgBytes...)
	err = i.SendMessage(peerAddress, tosend)
	if err != nil {
		log.Printf("sendDecision SendMessage error: %v\n", err)
	}
}

func (i *Consensus) AcceptChoice(choice Choice) {
	undecided := true
	term := choice.Term
	lastChoice := choice.Color
	ctx, cancel := context.WithTimeout(context.Background(), i.M)
	defer cancel()
	cnt := 0                       //number of successive aggreement on current value
	confidence := map[string]int{} //number of accrued chits, used to switch choice

	//SnowballLoop
	for undecided {
		//after timer M, no matter which choice the instance is holding, go with it
		select {
		case <-ctx.Done():
			log.Printf("%v timeout loop query for term %v with cnt=%d\n", i.MyAddress(), choice.Term, cnt)
			return
		default:
		}
		//sample K peers
		samples := i.sampleK()
		//for each peer in sample, do query for the choice
		choiceCount := i.querySampeK(choice, samples)
		//loop check if choiceCount contains any choice that has the majority votes
		majority := false
		for choice, count := range choiceCount {
			if count < i.A {
				continue
			}

			majority = true

			confidence[choice]++
			if confidence[choice] > confidence[i.Values[term]] {
				i.switchValue(Choice{Term: term, Color: choice})
			}

			if choice != lastChoice {
				lastChoice = choice
				cnt = 1
			} else {
				cnt++
			}

			if cnt >= i.B {
				i.switchValue(Choice{Term: term, Color: lastChoice})
				undecided = false
			}
		}
		if !majority {
			cnt = 0
		}
	}
}

// switchValue switches value and sends notification
func (i *Consensus) switchValue(choice Choice) {
	i.valuesLock.Lock()
	i.Values[choice.Term] = choice.Color
	i.valuesLock.Unlock()
	msg := Message{
		Type:    CHANGE,
		Address: i.MyAddress(),
		Data:    choice,
	}
	go func() {
		err := i.noti.NotifyChange(i.Node, msg)
		if err != nil {
			log.Printf("NotifyChange error: %v\n", err)
		}
	}()
}

// sampleK picks k random distinct nodes in peer list
func (i *Consensus) sampleK() map[string]bool {
	allPeers := i.GetPeers()
	allPeersCount := len(allPeers)
	samples := map[string]bool{}
	for len(samples) < i.K {
		index := rand.Intn(allPeersCount)
		peer := allPeers[index]
		if peer == i.noti.MyAddress() { //notification node doesnt need to join the voting quorum
			continue
		}
		samples[peer] = true
	}
	return samples
}

// querySampeK ask k peers which color do they choose, then return votes
// for each color
func (i *Consensus) querySampeK(choice Choice, samples map[string]bool) (
	result map[string]int) {
	choiceCount := map[string]int{}

	//make a chan to receive results from other peers
	choices := make(chan Choice, i.K)
	id := uuid.New().String()
	i.queryResultQueueLock.Lock()
	i.queryResultQueue[id] = choices
	i.queryResultQueueLock.Unlock()

	//query
	for peer := range samples {
		go i.query(peer, id, choice)
	}

	//handle query result
	received := 0
	ctx, cancel := context.WithTimeout(context.Background(), i.TimeoutQuerySampleK)
	defer cancel()
WAIT_AND_RECORD:
	for {
		select {
		case choice := <-choices:
			choiceCount[choice.Color]++
			received++
			if received == i.K {
				break WAIT_AND_RECORD
			}
		case <-ctx.Done():
			// log.Printf("%v timeout query sample K for term %v\n", i.MyAddress(), choice.Term)
			break WAIT_AND_RECORD
		}
	}
	i.queryResultQueueLock.Lock()
	delete(i.queryResultQueue, id)
	i.queryResultQueueLock.Unlock()
	// fmt.Printf("%v: choiceCount: %v\n", id, choiceCount)
	return choiceCount
}

// query sends QUERY message to peerAddress
func (i *Consensus) query(peerAddress string, id string, choice Choice) {
	msg := Message{
		ID:      id,
		Address: i.MyAddress(),
		Type:    QUERY,
		Data:    choice,
	}
	msgBytes, err := json.Marshal(&msg)
	if err != nil {
		log.Printf("Consensus Consensus query error: %v\n", err)
		return
	}
	tosend := append([]byte{FIRST_BIT}, msgBytes...)
	err = i.SendMessage(peerAddress, tosend)
	if err != nil {
		log.Printf("query SendMessage error: %v\n", err)
	}
}

// onMessage is the custom handler to plug into P2pNode
func (i *Consensus) onMessage(msgBytes []byte) {
	if msgBytes[0] != FIRST_BIT {
		return
	}
	var msg Message
	err := json.Unmarshal(msgBytes[1:], &msg)
	if err != nil {
		log.Println(err)
		return
	}
	switch msg.Type {
	case QUERY:
		i.OnQuery(msg)
	case QUERY_RESULT:
		i.queryResultQueueLock.Lock()
		ch := i.queryResultQueue[msg.ID]
		i.queryResultQueueLock.Unlock()
		ch <- msg.Data
	case GENERATE:
		i.AcceptChoice(msg.Data)
	}
}
