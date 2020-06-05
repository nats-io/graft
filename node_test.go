// Copyright 2013-2020 The NATS Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package graft

import (
	"runtime"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// Test bad ClusterInfos
	bci := ClusterInfo{Name: "", Size: 5}
	if _, err := New(bci, nil, nil, ""); err == nil || err != ErrClusterName {
		t.Fatal("Expected an error with empty cluster name")
	}
	bci = ClusterInfo{Name: "foo", Size: 0}
	if _, err := New(bci, nil, nil, ""); err == nil || err != ErrClusterSize {
		t.Fatal("Expected an error with empty cluster name")
	}

	// Good ClusterInfo
	ci := ClusterInfo{Name: "foo", Size: 3}

	// Handler is required
	if _, err := New(ci, nil, nil, ""); err == nil || err != ErrHandlerReq {
		t.Fatal("Expected an error with no handler argument")
	}

	hand, rpc, log := genNodeArgs(t)

	// rpcDriver is required
	if _, err := New(ci, hand, nil, ""); err == nil || err != ErrRpcDriverReq {
		t.Fatal("Expected an error with no rpcDriver argument")
	}

	// Test if rpc Init fails we get error from New()
	badRpc := &MockRpcDriver{shouldFailInit: true}
	if _, err := New(ci, hand, badRpc, ""); err == nil {
		t.Fatal("Expected an error with a bad rpcDriver argument")
	}

	// Test peer count
	mpc := mockPeerCount()
	if mpc != 0 {
		t.Fatalf("Incorrect peer count, expected 0 got %d\n", mpc)
	}

	// log is required
	if _, err := New(ci, hand, rpc, ""); err == nil || err != ErrLogReq {
		t.Fatal("Expected an error with no log argument")
	}

	node, err := New(ci, hand, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	defer node.Close()

	// Check default state
	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected new Node to be in Follower state, got: %s", state)
	}
	// Check string version of state
	if stateStr := node.State().String(); stateStr != "Follower" {
		t.Fatalf("Expected new Node to be in Follower state, got: %s", stateStr)
	}

	if node.Leader() != NO_LEADER {
		t.Fatalf("Expected no leader to start, got: %s\n", node.Leader())
	}
	if node.CurrentTerm() != 0 {
		t.Fatalf("Expected CurrentTerm of 0, got: %d\n", node.CurrentTerm())
	}
}

func TestClose(t *testing.T) {
	base := runtime.NumGoroutine()

	ci := ClusterInfo{Name: "foo", Size: 3}
	hand, rpc, log := genNodeArgs(t)

	node, err := New(ci, hand, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}

	node.Close()

	if node.isRunning() {
		t.Fatal("Expected isRunning() to return false")
	}

	// Check state
	if state := node.State(); state != CLOSED {
		t.Fatalf("Expected node to be in Closed state, got: %s", state)
	}
	if stateStr := node.State().String(); stateStr != "Closed" {
		t.Fatalf("Expected node to be in Closed state, got: %s", stateStr)
	}

	// Check to make sure rpc.Close() was called.
	if rawRpc := rpc.(*MockRpcDriver); !rawRpc.closeCalled {
		t.Fatalf("RPCDriver was not shutdown properly")
	}

	// Make sure the timers were cleared.
	if node.electTimer != nil {
		t.Fatalf("electTimer was not cleared")
	}

	// Check for dangling go routines
	delta := (runtime.NumGoroutine() - base)
	if delta > 0 {
		t.Fatalf("[%d] Go routines still exist post Close()", delta)
	}
}

func TestElectionTimeoutDuration(t *testing.T) {
	et := randElectionTimeout()
	if et < MIN_ELECTION_TIMEOUT || et > MAX_ELECTION_TIMEOUT {
		t.Fatalf("Election Timeout expected to be between %d-%d ms, got %d ms",
			MIN_ELECTION_TIMEOUT/time.Millisecond,
			MAX_ELECTION_TIMEOUT/time.Millisecond,
			et/time.Millisecond)
	}
}

func TestCandidateState(t *testing.T) {
	ci := ClusterInfo{Name: "foo", Size: 3}
	hand, rpc, log := genNodeArgs(t)
	node, err := New(ci, hand, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	defer node.Close()

	// Should move to candidate state within MAX_ELECTION_TIMEOUT
	if state := waitForState(node, CANDIDATE); state != CANDIDATE {
		t.Fatalf("Expected node to move to Candidate state, got: %s", state)
	}
	if stateStr := node.State().String(); stateStr != "Candidate" {
		t.Fatalf("Expected node to move to Candidate state, got: %s", stateStr)
	}
}

func TestLeaderState(t *testing.T) {
	// Expected of 1, we should immediately win the election.
	ci := ClusterInfo{Name: "foo", Size: 1}
	hand, rpc, log := genNodeArgs(t)
	node, err := New(ci, hand, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	defer node.Close()

	// Should move to leader state within MAX_ELECTION_TIMEOUT
	if state := waitForState(node, LEADER); state != LEADER {
		t.Fatalf("Expected node to move to Leader state, got: %s", state)
	}
	if stateStr := node.State().String(); stateStr != "Leader" {
		t.Fatalf("Expected node to move to Leader state, got: %s", stateStr)
	}
}

func TestSimpleLeaderElection(t *testing.T) {
	toStart := 5
	nodes := createNodes(t, "foo", toStart)
	// Do cleanup
	for _, n := range nodes {
		defer n.Close()
	}

	expectedClusterState(t, nodes, 1, toStart-1, 0)
}

func TestStaggeredStart(t *testing.T) {
	ci := ClusterInfo{Name: "staggered", Size: 3}
	nodes := make([]*Node, 3)
	for i := 0; i < 3; i++ {
		hand, rpc, logPath := genNodeArgs(t)
		node, err := New(ci, hand, rpc, logPath)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		nodes[i] = node
		time.Sleep(MAX_ELECTION_TIMEOUT)
	}
	// Do cleanup
	for _, n := range nodes {
		defer n.Close()
	}
	expectedClusterState(t, nodes, 1, 2, 0)
}

func TestDownToOneAndBack(t *testing.T) {
	nodes := createNodes(t, "downtoone", 3)
	expectedClusterState(t, nodes, 1, 2, 0)

	// Do cleanup
	for _, n := range nodes {
		defer n.Close()
	}

	// find and kill the leader
	leader := findLeader(nodes)
	leader.Close()
	expectedClusterState(t, nodes, 1, 1, 0)

	// start a new process in the leader's place
	leader = findLeader(nodes)
	follower := firstFollower(nodes)
	nodes = []*Node{leader, follower}
	hand, rpc, logPath := genNodeArgs(t)
	newNode, err := New(leader.ClusterInfo(), hand, rpc, logPath)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	defer newNode.Close()
	nodes = append(nodes, newNode)
	expectedClusterState(t, nodes, 1, 2, 0)

	// find and kill the new leader
	leader = findLeader(nodes)
	leader.Close()
	expectedClusterState(t, nodes, 1, 1, 0)

	// find the leader again and kill it
	leader = findLeader(nodes)
	leader.Close()
	expectedClusterState(t, nodes, 0, 0, 1)

	// grab the surviving node, we'll want to compare term numbers
	var survivingNode *Node
	for _, n := range nodes {
		if n.State() == CANDIDATE {
			survivingNode = n
			break
		}
	}
	if survivingNode == nil {
		t.Fatal("Failed to find the surving node")
	}

	// start the two other nodes back up
	nodes = []*Node{survivingNode}
	for i := 0; i < 2; i++ {
		hand, rpc, logPath := genNodeArgs(t)
		node, err := New(survivingNode.ClusterInfo(), hand, rpc, logPath)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		nodes = append(nodes, node)
		defer node.Close()
	}

	// we expect to be in a consistent state and for terms to match
	expectedClusterState(t, nodes, 1, 2, 0)
	leader = findLeader(nodes)
	if leader.CurrentTerm() != survivingNode.CurrentTerm() {
		t.Fatalf("term between leader and survivor didn't match. leader: %d, survivor: %d", leader.CurrentTerm(), survivingNode.CurrentTerm())
	}
}

func TestReElection(t *testing.T) {
	toStart := 5
	nodes := createNodes(t, "foo", toStart)
	// Do cleanup
	for _, n := range nodes {
		defer n.Close()
	}

	time.Sleep(MAX_ELECTION_TIMEOUT)

	// Find and close down the leader
	leader := findLeader(nodes)
	if leader == nil {
		t.Fatal("Could not find a leader!\n")
	}
	leader.Close()

	// Make sure we have another leader.
	expectedClusterState(t, nodes, 1, toStart-2, 0)
}

func TestNetworkSplit(t *testing.T) {
	clusterSize := 5

	nodes := createNodes(t, "foo", clusterSize)
	// Do cleanup
	for _, n := range nodes {
		defer n.Close()
	}

	// Make sure we have correct count.
	expectedClusterState(t, nodes, 1, clusterSize-1, 0)

	// Simulate a network split. We will pick the leader and 1 follower
	// to be in one group, all others will be in the other.

	theLeader := findLeader(nodes)
	if theLeader == nil {
		t.Fatal("Expected to find a leader, got <nil>")
	}
	aFollower := firstFollower(nodes)
	if aFollower == nil {
		t.Fatal("Expected to find a follower, got <nil>")
	}
	grp := []*Node{theLeader, aFollower}

	// Split the nodes in two..
	mockSplitNetwork(grp)

	// Make sure we have another leader.
	expectedClusterState(t, nodes, 2, clusterSize-2, 0)

	// Restore Communications
	mockRestoreNetwork()

	expectedClusterState(t, nodes, 1, clusterSize-1, 0)
}
