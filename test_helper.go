// Copyright 2013 Apcera Inc. All rights reserved.

package graft

import (
	"io/ioutil"
	"testing"
)

type dummyHandler struct {
}

func (*dummyHandler) AsyncError(err error)       {}
func (*dummyHandler) StateChange(from, to State) {}

func genNodeArgs(t *testing.T) (Handler, RPCDriver, string) {
	hand := &dummyHandler{}
	rpc := NewMockRpc()
	log, err := ioutil.TempFile("", "_grafty_log")
	if err != nil {
		t.Fatal("Could not create the log file")
	}
	defer log.Close()
	return hand, rpc, log.Name()
}

func createNodes(t *testing.T, name string, numNodes int) []*Node {
	ci := ClusterInfo{Name: name, Size: numNodes}
	nodes := make([]*Node, numNodes)
	for i := 0; i < numNodes; i++ {
		hand, rpc, logPath := genNodeArgs(t)
		node, err := New(ci, hand, rpc, logPath)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		nodes[i] = node
	}
	return nodes
}

func countTypes(nodes []*Node) (leaders, followers, candidates int) {
	for _, n := range nodes {
		switch n.State() {
		case LEADER:
			leaders++
		case FOLLOWER:
			followers++
		case CANDIDATE:
			candidates++
		}
	}
	return
}

func findLeader(nodes []*Node) *Node {
	for _, n := range nodes {
		if n.State() == LEADER {
			return n
		}
	}
	return nil
}
