// Copyright 2013 Apcera Inc. All rights reserved.

package graft

import (
	"errors"
	"sync"
)

var mu sync.Mutex
var peers map[string]*Node

func init() {
	peers = make(map[string]*Node)
}

func mockPeerCount() int {
	mu.Lock()
	defer mu.Unlock()
	return len(peers)
}

func mockPeers() []*Node {
	mu.Lock()
	defer mu.Unlock()
	nodes := make([]*Node, 0, len(peers))
	for _, p := range peers {
		nodes = append(nodes, p)
	}
	return nodes
}

func mockRegisterPeer(n *Node) {
	mu.Lock()
	defer mu.Unlock()
	peers[n.id] = n
}

func mockUnregisterPeer(id string) {
	mu.Lock()
	defer mu.Unlock()
	delete(peers, id)
}

type MockRpcDriver struct {
	mu   sync.Mutex
	node *Node

	// For testing
	shouldFailInit bool
	closeCalled    bool
	shouldFailComm bool
}

func NewMockRpc() *MockRpcDriver {
	return &MockRpcDriver{}
}

func (rpc *MockRpcDriver) Init(n *Node) error {
	if rpc.shouldFailInit {
		return errors.New("RPC Failed to Init")
	}
	// Redo the channels to be buffered since we could be
	// sending and block the select loops.
	cSize := n.ClusterInfo().Size
	n.VoteRequests = make(chan *VoteRequest, cSize)
	n.VoteResponses = make(chan *VoteResponse, cSize)
	n.HeartBeats = make(chan *Heartbeat, cSize)

	mockRegisterPeer(n)
	rpc.node = n
	return nil
}

func (rpc *MockRpcDriver) Close() {
	rpc.closeCalled = true
	if rpc.node != nil {
		mockUnregisterPeer(rpc.node.id)
	}
}

func (rpc *MockRpcDriver) RequestVote(vr *VoteRequest) error {
	if rpc.isCommBlocked() {
		// Silent failure
		return nil
	}
	for _, p := range mockPeers() {
		if p.id != rpc.node.id {
			p.VoteRequests <- vr
		}
	}
	return nil
}

func (rpc *MockRpcDriver) HeartBeat(hb *Heartbeat) error {
	if rpc.isCommBlocked() {
		// Silent failure
		return nil
	}

	for _, p := range mockPeers() {
		if p.id != rpc.node.id {
			p.HeartBeats <- hb
		}
	}
	return nil
}

func (rpc *MockRpcDriver) SendVoteResponse(candidate string, vresp *VoteResponse) error {
	if rpc.isCommBlocked() {
		// Silent failure
		return nil
	}

	mu.Lock()
	p := peers[candidate]
	mu.Unlock()

	if p != nil && p.isRunning() {
		p.VoteResponses <- vresp
	}
	return nil
}

func (rpc *MockRpcDriver) setCommBlocked(block bool) {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()
	rpc.shouldFailComm = block
}

func (rpc *MockRpcDriver) isCommBlocked() bool {
	rpc.mu.Lock()
	defer rpc.mu.Unlock()
	return rpc.shouldFailComm
}