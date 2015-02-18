// Copyright 2013-2014 Apcera Inc. All rights reserved.

package graft

import (
	"runtime"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	// Test bad ClusterInfos
	bci := ClusterInfo{Name: "", Size: 5}
	if _, err := New(bci, nil, nil, ""); err == nil || err != ClusterNameErr {
		t.Fatal("Expected an error with empty cluster name")
	}
	bci = ClusterInfo{Name: "foo", Size: 0}
	if _, err := New(bci, nil, nil, ""); err == nil || err != ClusterSizeErr {
		t.Fatal("Expected an error with empty cluster name")
	}

	// Good ClusterInfo
	ci := ClusterInfo{Name: "foo", Size: 3}

	// Handler is required
	if _, err := New(ci, nil, nil, ""); err == nil || err != HandlerReqErr {
		t.Fatal("Expected an error with no handler argument")
	}

	hand, rpc, log := genNodeArgs(t)

	// rpcDriver is required
	if _, err := New(ci, hand, nil, ""); err == nil || err != RpcDriverReqErr {
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
	if _, err := New(ci, hand, rpc, ""); err == nil || err != LogReqErr {
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
	time.Sleep(MAX_ELECTION_TIMEOUT)
	if state := node.State(); state != CANDIDATE {
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
	time.Sleep(MAX_ELECTION_TIMEOUT)
	if state := node.State(); state != LEADER {
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

	time.Sleep(MAX_ELECTION_TIMEOUT)

	leaders, followers, candidates := countTypes(nodes)

	if leaders != 1 {
		t.Fatalf("Expected 1 Leader, got %d\n", leaders)
	}
	if followers != toStart-1 {
		t.Fatalf("Expected %d Followers, got %d\n", toStart-1, followers)
	}
	if candidates != 0 {
		t.Fatalf("Expected 0 Candidates, got %d\n", candidates)
	}
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
		t.Logf("Started node %d", i+1)
		time.Sleep(2 * MAX_ELECTION_TIMEOUT)
	}

	leaders, followers, candidates := countTypes(nodes)

	if leaders != 1 {
		t.Fatalf("Expected 1 Leader, got %d Leaders, %d followers, %d candidates\n", leaders, followers, candidates)
	}
	if followers != 2 {
		t.Fatalf("Expected 2 Follers, got %d Leaders, %d followers, %d candidates\n", leaders, followers, candidates)
	}
	if candidates != 0 {
		t.Fatalf("Expected 0 Candidates, got %d Leaders, %d followers, %d candidates\n", leaders, followers, candidates)
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
	time.Sleep(MAX_ELECTION_TIMEOUT)

	// Make sure we have another leader.
	leaders, followers, candidates := countTypes(nodes)

	if leaders != 1 {
		t.Fatalf("Expected 1 Leader, got %d\n", leaders)
	}
	if followers != toStart-2 {
		t.Fatalf("Expected %d Followers, got %d\n", toStart-2, followers)
	}
	if candidates != 0 {
		t.Fatalf("Expected 0 Candidates, got %d\n", candidates)
	}
}

func TestNetworkSplit(t *testing.T) {
	clusterSize := 5

	nodes := createNodes(t, "foo", clusterSize)
	// Do cleanup
	for _, n := range nodes {
		defer n.Close()
	}

	time.Sleep(MAX_ELECTION_TIMEOUT)

	// Make sure we have correct count.
	leaders, followers, _ := countTypes(nodes)

	if leaders != 1 {
		t.Fatal("Expected a leader")
	}
	expectedFollowers := clusterSize - 1
	if followers != expectedFollowers {
		t.Fatalf("Expected %d followers, got %d",
			expectedFollowers, followers)
	}

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

	// Wait on election timeout
	time.Sleep(MAX_ELECTION_TIMEOUT)

	// Make sure we have another leader.
	leaders, followers, _ = countTypes(nodes)

	if leaders != 2 {
		t.Fatalf("Expected 2 leaders, got %d\n", leaders)
	}

	// Restore Communications
	mockRestoreNetwork()

	time.Sleep(MAX_ELECTION_TIMEOUT)

	leaders, followers, _ = countTypes(nodes)
	if leaders != 1 {
		t.Fatal("Expected a leader")
	}
	if followers != expectedFollowers {
		t.Fatalf("Expected %d followers, got %d",
			expectedFollowers, followers)
	}
}
