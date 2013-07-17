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
	node *Node

	// For testing
	shouldFailInit bool
	closeCalled    bool
}

func NewMockRpc() *MockRpcDriver {
	return &MockRpcDriver{}
}

func (rpc *MockRpcDriver) Init(n *Node) error {
	if rpc.shouldFailInit {
		return errors.New("RPC Failed to Init")
	}
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
	mu.Lock()
	defer mu.Unlock()
	for _, p := range peers {
		if p.id != rpc.node.id {
			p.VoteRequests <- vr
		}
	}
	return nil
}

func (rpc *MockRpcDriver) HeartBeat(hb *Heartbeat) error {
	mu.Lock()
	defer mu.Unlock()
	for _, p := range peers {
		if p.id != rpc.node.id {
			p.HeartBeats <- hb
		}
	}
	return nil
}

func (rpc *MockRpcDriver) SendVoteResponse(candidate string, vresp *VoteResponse) error {
	mu.Lock()
	defer mu.Unlock()
	if p := peers[candidate]; p != nil {
		p.VoteResponses <- vresp
	}
	return nil
}
