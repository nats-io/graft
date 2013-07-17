// Copyright 2013 Apcera Inc. All rights reserved.

package graft

import (
	"testing"
	"time"
)

// Test VoteRequests RPC in different states.

func vreqNode(t *testing.T, expected int) *Node {
	ci := ClusterInfo{Name: "vreq", Size: expected}
	hand, rpc, log := genNodeArgs(t)
	node, err := New(ci, hand, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	// Delay elections
	node.electTimer.Reset(10 * time.Second)
	return node
}

func fakeNode(name string) *Node {
	// Create fake node to watch VoteResponses.
	fake := &Node{id: name}
	fake.VoteResponses = make(chan *VoteResponse)
	fake.VoteRequests = make(chan *VoteRequest, 32)
	return fake
}

func TestVoteRequestAsFollower(t *testing.T) {
	node := vreqNode(t, 3)
	defer node.Close()

	// Set term to artificial higher value.
	newTerm := uint64(8)
	node.setTerm(newTerm)

	// Force write of state
	node.writeState()

	// Create fake node to watch VoteResponses.
	fake := fakeNode("fake")

	// Hook up to MockRPC layer
	mockRegisterPeer(fake)
	defer mockUnregisterPeer(fake.id)

	// a VoteRequest with lower term should be ignored
	node.VoteRequests <- &VoteRequest{Term: 1, Candidate: fake.id}
	vresp := <-fake.VoteResponses
	if vresp.Term != newTerm {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			newTerm, vresp.Term)
	}
	if vresp.Granted != false {
		t.Fatal("Expected the VoteResponse to have Granted of false")
	}
	// Make sure no changes to node
	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if node.Leader() != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", node.Leader())
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}

	// Test persistent state
	testStateOfNode(t, node)

	// a VoteRequest with a higher term should reset follower
	newTerm++

	node.VoteRequests <- &VoteRequest{Term: newTerm, Candidate: fake.id}
	vresp = <-fake.VoteResponses
	if vresp.Term != newTerm {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			newTerm, vresp.Term)
	}
	if vresp.Granted == false {
		t.Fatal("Expected the VoteResponse to have been Granted")
	}
	// Verify new state
	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if node.Leader() != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", node.Leader())
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}
	if node.vote == NO_VOTE {
		t.Fatal("Expected to have a cast vote at this point")
	}

	// Test persistent state
	testStateOfNode(t, node)
}

func TestVoteRequestAsCandidate(t *testing.T) {
	node := vreqNode(t, 3)
	defer node.Close()

	// Set term to artificial higher value.
	newTerm := uint64(8)
	node.setTerm(newTerm)

	// Force write of state
	node.writeState()

	// Create fake node to watch VoteResponses.
	fake := fakeNode("fake")

	// Hook up to MockRPC layer
	mockRegisterPeer(fake)
	defer mockUnregisterPeer(fake.id)

	node.electTimer.Reset(1 * time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	newTerm++

	// Verify new state
	if state := node.State(); state != CANDIDATE {
		t.Fatalf("Expected Node to be in Candidate state, got: %s", state)
	}
	if node.Leader() != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", node.Leader())
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}
	if vote := node.CurrentVote(); vote != node.id {
		t.Fatal("Expected Node to have a cast vote for itself, got:%s", vote)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// a VoteRequest with lower term should be ignored
	node.VoteRequests <- &VoteRequest{Term: 1, Candidate: fake.id}
	vresp := <-fake.VoteResponses
	if vresp.Term != newTerm {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			newTerm, vresp.Term)
	}
	if vresp.Granted != false {
		t.Fatal("Expected the VoteResponse to have Granted of false")
	}
	// Make sure no changes to node
	if state := node.State(); state != CANDIDATE {
		t.Fatalf("Expected Node to be in Candidate state, got: %s", state)
	}
	if node.Leader() != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", node.Leader())
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}
	if vote := node.CurrentVote(); vote != node.id {
		t.Fatal("Expected Node to have still cast vote for itself, got:%s", vote)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// a VoteRequest for same term but different candidate should be
	// denied since we always vote for ourself.
	node.VoteRequests <- &VoteRequest{Term: newTerm, Candidate: fake.id}
	vresp = <-fake.VoteResponses
	if vresp.Term != newTerm {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			newTerm, vresp.Term)
	}
	if vresp.Granted != false {
		t.Fatal("Expected the VoteResponse to have Granted of false")
	}
	// Make sure no changes to node
	if state := node.State(); state != CANDIDATE {
		t.Fatalf("Expected Node to be in Candidate state, got: %s", state)
	}
	if node.Leader() != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", node.Leader())
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}
	if vote := node.CurrentVote(); vote != node.id {
		t.Fatal("Expected Node to have still cast vote for itself, got:%s", vote)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// a VoteRequest with a higher term should reset to follower
	newTerm++
	node.VoteRequests <- &VoteRequest{Term: newTerm, Candidate: fake.id}
	vresp = <-fake.VoteResponses
	if vresp.Term != newTerm {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			newTerm, vresp.Term)
	}
	if vresp.Granted == false {
		t.Fatal("Expected the VoteResponse to have been Granted")
	}
	// Verify new state
	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if node.Leader() != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", node.Leader())
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}
	if vote := node.CurrentVote(); vote != fake.id {
		t.Fatalf("Expected to have voted for %s, got %s", fake.id, node.vote)
	}

	// Test persistent state
	testStateOfNode(t, node)
}

func TestVoteRequestAsLeader(t *testing.T) {
	node := vreqNode(t, 3)
	defer node.Close()

	// Set term to artificial higher value.
	newTerm := uint64(8)
	node.setTerm(newTerm)

	// Force write of state
	node.writeState()

	// Create fake node to watch VoteResponses.
	fake := fakeNode("fake")

	// Hook up to MockRPC layer
	mockRegisterPeer(fake)
	defer mockUnregisterPeer(fake.id)

	newTerm++

	node.electTimer.Reset(1 * time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	vreq := <-fake.VoteRequests

	// Send Fake VoteResponse to promote node to Leader
	node.VoteResponses <- &VoteResponse{Term: vreq.Term, Granted: true}

	// FIXME(dlc) use handler instead.
	time.Sleep(5 * time.Millisecond)

	if state := node.State(); state != LEADER {
		t.Fatalf("Expected Node to be in Leader state, got: %s", state)
	}
	if node.Leader() != node.id {
		t.Fatalf("Expected us to be leader, got: %s\n", node.Leader())
	}
	if node.CurrentTerm() != vreq.Term {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			vreq.Term, node.CurrentTerm())
	}

	// Test persistent state
	testStateOfNode(t, node)

	// a VoteRequest with lower term should be ignored
	node.VoteRequests <- &VoteRequest{Term: 1, Candidate: fake.id}
	vresp := <-fake.VoteResponses
	if vresp.Term != newTerm {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			newTerm, vresp.Term)
	}
	if vresp.Granted != false {
		t.Fatal("Expected the VoteResponse to have Granted of false")
	}
	// Make sure no changes to node
	if state := node.State(); state != LEADER {
		t.Fatalf("Expected Node to be in Leader state, got: %s", state)
	}
	if node.Leader() != node.id {
		t.Fatalf("Expected us to be leader, got: %s\n", node.Leader())
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}

	// Test persistent state
	testStateOfNode(t, node)

	// a VoteRequest with a higher term should force us to stepdown
	newTerm++
	node.VoteRequests <- &VoteRequest{Term: newTerm, Candidate: fake.id}
	vresp = <-fake.VoteResponses
	if vresp.Term != newTerm {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			newTerm, vresp.Term)
	}
	if vresp.Granted == false {
		t.Fatal("Expected the VoteResponse to have been Granted")
	}
	// Verify new state
	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if node.Leader() != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", node.Leader())
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}
	if vote := node.CurrentVote(); vote != fake.id {
		t.Fatalf("Expected to have voted for %s, got %s", fake.id, node.vote)
	}

	// Test persistent state
	testStateOfNode(t, node)
}
