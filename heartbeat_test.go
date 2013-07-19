// Copyright 2013 Apcera Inc. All rights reserved.

package graft

import (
	"testing"
	"time"
)

// Test HeartBeat RPC in different states.

func hbNode(t *testing.T, expected int) *Node {
	ci := ClusterInfo{Name: "ae", Size: expected}
	hand, rpc, log := genNodeArgs(t)
	node, err := New(ci, hand, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	// Delay elections
	node.electTimer.Reset(10 * time.Second)
	return node
}

func sendAndWait(n *Node, hb *Heartbeat) {
	// Any Heartbeat will set the Leader if none given.
	n.HeartBeats <- hb
	// Wait for AE to be processed.
	for len(n.HeartBeats) > 0 {
		time.Sleep(1 * time.Millisecond)
	}
}

func TestHeartBeatAsFollower(t *testing.T) {
	node := hbNode(t, 3)
	defer node.Close()

	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", curLeader)
	}

	leader := "leader123"
	term := uint64(2)

	// Any HeartBeat will set the Leader if none given.
	sendAndWait(node, &Heartbeat{Term: term, Leader: leader})

	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != leader {
		t.Fatalf("Expected leader to be %s, got: %s\n", leader, curLeader)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// Ignore lower terms when we have a leader already.
	// any HeartBeat will set the Leader if none given.
	oldLeader := "oldleader"
	sendAndWait(node, &Heartbeat{Term: 1, Leader: oldLeader})

	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != leader {
		t.Fatalf("Expected leader to be %s, got: %s\n", leader, curLeader)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// A newer term will reset.
	newLeader := "newLeader"
	newTerm := uint64(10)
	sendAndWait(node, &Heartbeat{Term: newTerm, Leader: newLeader})

	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != newLeader {
		t.Fatalf("Expected leader to be %s, got: %s\n", newLeader, curLeader)
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}

	// Test persistent state
	testStateOfNode(t, node)
}

func TestHeartBeatAsCandidate(t *testing.T) {
	node := hbNode(t, 3)
	defer node.Close()

	// Speed up switch to Candidate state by shortening the timer.
	node.electTimer.Reset(1 * time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	// Verify new state
	if state := node.State(); state != CANDIDATE {
		t.Fatalf("Expected Node to be in Candidate state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", curLeader)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// Set term to artificial higher value.
	newTerm := uint64(8)
	node.setTerm(newTerm)

	// Ignore HeartBeat with lower terms.
	leader := "leader123"
	term := uint64(2)

	// Any HeartBeat will set the Leader if none given.
	sendAndWait(node, &Heartbeat{Term: term, Leader: leader})

	if state := node.State(); state != CANDIDATE {
		t.Fatalf("Expected Node to be in Candidate state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", curLeader)
	}

	// A newer term will reset us to a follower.
	newLeader := "newLeader"
	newTerm++

	sendAndWait(node, &Heartbeat{Term: newTerm, Leader: newLeader})

	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != newLeader {
		t.Fatalf("Expected leader to be %s, got: %s\n", newLeader, curLeader)
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}
	if vote := node.CurrentVote(); vote != NO_VOTE {
		t.Fatal("Expected to have no vote at this point, got %s", node.vote)
	}

	// Test persistent state
	testStateOfNode(t, node)
}

func TestHeartBeatAsLeader(t *testing.T) {
	node := hbNode(t, 3)
	defer node.Close()

	// Set term to artificial higher value.
	newTerm := uint64(8)
	node.setTerm(newTerm)

	// Create fake node to elect the Leader.
	fake := fakeNode("fake")

	// Hook up to MockRPC layer
	mockRegisterPeer(fake)
	defer mockUnregisterPeer(fake.id)

	node.electTimer.Reset(1 * time.Millisecond)
	time.Sleep(5 * time.Millisecond)

	// Verify new state
	if state := node.State(); state != CANDIDATE {
		t.Fatalf("Expected Node to be in Candidate state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", curLeader)
	}

	vreq := <-fake.VoteRequests

	// Send Fake VoteResponse to promote node to Leader
	node.VoteResponses <- &VoteResponse{Term: vreq.Term, Granted: true}

	time.Sleep(5 * time.Millisecond)

	if state := node.State(); state != LEADER {
		t.Fatalf("Expected Node to be in Leader state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != node.Id() {
		t.Fatalf("Expected us to be leader, got: %s\n", curLeader)
	}
	if node.CurrentTerm() != vreq.Term {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			vreq.Term, node.CurrentTerm())
	}

	// Test persistent state
	testStateOfNode(t, node)

	// Ignore HeartBeat with lower terms.
	leader := "leader123"
	term := uint64(2)

	// Any HeartBeat will set the Leader if none given.
	sendAndWait(node, &Heartbeat{Term: term, Leader: leader})

	if state := node.State(); state != LEADER {
		t.Fatalf("Expected Node to be in Leader state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != node.Id() {
		t.Fatalf("Expected us to be leader, got: %s\n", curLeader)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// A newer term will reset us to a follower.
	newLeader := "newLeader"
	newTerm = 20

	sendAndWait(node, &Heartbeat{Term: newTerm, Leader: newLeader})

	if state := node.State(); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := node.Leader(); curLeader != newLeader {
		t.Fatalf("Expected leader to be %s, got: %s\n", newLeader, curLeader)
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}
	if vote := node.CurrentVote(); vote != NO_VOTE {
		t.Fatal("Expected to have no vote at this point, got %s", node.vote)
	}

	// Test persistent state
	testStateOfNode(t, node)
}
