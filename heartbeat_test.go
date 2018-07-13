// Copyright 2013-2018 The NATS Authors
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
	"os"
	"testing"
	"time"

	"github.com/nedscode/graft/pb"
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

func sendAndWait(n *Node, hb *pb.Heartbeat) {
	// Any Heartbeat will set the Leader if none given.
	n.HeartBeats <- hb
	// Wait for AE to be processed.
	for len(n.HeartBeats) > 0 {
		time.Sleep(10 * time.Millisecond)
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
	sendAndWait(node, &pb.Heartbeat{Term: term, Leader: leader})

	if state := waitForState(node, FOLLOWER); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := waitForLeader(node, leader); curLeader != leader {
		t.Fatalf("Expected leader to be %s, got: %s\n", leader, curLeader)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// Ignore lower terms when we have a leader already.
	// any HeartBeat will set the Leader if none given.
	oldLeader := "oldleader"
	sendAndWait(node, &pb.Heartbeat{Term: 1, Leader: oldLeader})

	if state := waitForState(node, FOLLOWER); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := waitForLeader(node, leader); curLeader != leader {
		t.Fatalf("Expected leader to be %s, got: %s\n", leader, curLeader)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// A newer term will reset.
	newLeader := "newLeader"
	newTerm := uint64(10)
	sendAndWait(node, &pb.Heartbeat{Term: newTerm, Leader: newLeader})

	if state := waitForState(node, FOLLOWER); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := waitForLeader(node, newLeader); curLeader != newLeader {
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
	time.Sleep(10 * time.Millisecond)

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
	sendAndWait(node, &pb.Heartbeat{Term: term, Leader: leader})

	if state := waitForState(node, CANDIDATE); state != CANDIDATE {
		t.Fatalf("Expected Node to be in Candidate state, got: %s", state)
	}
	if curLeader := waitForLeader(node, NO_LEADER); curLeader != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", curLeader)
	}

	// A newer term will reset us to a follower.
	newLeader := "newLeader"
	newTerm++

	sendAndWait(node, &pb.Heartbeat{Term: newTerm, Leader: newLeader})

	if state := waitForState(node, FOLLOWER); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := waitForLeader(node, newLeader); curLeader != newLeader {
		t.Fatalf("Expected leader to be %s, got: %s\n", newLeader, curLeader)
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}
	if vote := node.CurrentVote(); vote != NO_VOTE {
		t.Fatalf("Expected to have no vote at this point, got %s", node.vote)
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

	// Verify new state
	if state := waitForState(node, CANDIDATE); state != CANDIDATE {
		t.Fatalf("Expected Node to be in Candidate state, got: %s", state)
	}
	if curLeader := waitForLeader(node, NO_LEADER); curLeader != NO_LEADER {
		t.Fatalf("Expected no leader, got: %s\n", curLeader)
	}

	vreq := <-fake.VoteRequests

	// Send Fake VoteResponse to promote node to Leader
	node.VoteResponses <- &pb.VoteResponse{Term: vreq.Term, Granted: true}

	if state := waitForState(node, LEADER); state != LEADER {
		t.Fatalf("Expected Node to be in Leader state, got: %s", state)
	}
	if curLeader := waitForLeader(node, node.Id()); curLeader != node.Id() {
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
	sendAndWait(node, &pb.Heartbeat{Term: term, Leader: leader})

	if state := waitForState(node, LEADER); state != LEADER {
		t.Fatalf("Expected Node to be in Leader state, got: %s", state)
	}
	if curLeader := waitForLeader(node, node.Id()); curLeader != node.Id() {
		t.Fatalf("Expected us to be leader, got: %s\n", curLeader)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// A newer term will reset us to a follower.
	newLeader := "newLeader"
	newTerm = 20

	sendAndWait(node, &pb.Heartbeat{Term: newTerm, Leader: newLeader})

	if state := waitForState(node, FOLLOWER); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
	if curLeader := waitForLeader(node, newLeader); curLeader != newLeader {
		t.Fatalf("Expected leader to be %s, got: %s\n", newLeader, curLeader)
	}
	if node.CurrentTerm() != newTerm {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			newTerm, node.CurrentTerm())
	}
	if vote := node.CurrentVote(); vote != NO_VOTE {
		t.Fatalf("Expected to have no vote at this point, got %s", node.vote)
	}

	// Test persistent state
	testStateOfNode(t, node)
}

func TestHeartBeatAsLeaderErrorOnWrite(t *testing.T) {
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

	node.mu.Lock()
	node.electTimer.Reset(1 * time.Millisecond)
	node.mu.Unlock()

	vreq := <-fake.VoteRequests

	// Send Fake VoteResponse to promote node to Leader
	node.VoteResponses <- &pb.VoteResponse{Term: vreq.Term, Granted: true}

	if state := waitForState(node, LEADER); state != LEADER {
		t.Fatalf("Expected Node to be in Leader state, got: %s", state)
	}
	if curLeader := waitForLeader(node, node.Id()); curLeader != node.Id() {
		t.Fatalf("Expected us to be leader, got: %s\n", curLeader)
	}
	if node.CurrentTerm() != vreq.Term {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			vreq.Term, node.CurrentTerm())
	}

	// Test persistent state
	testStateOfNode(t, node)

	// Change log permissions to cause error
	if err := os.Chmod(node.logPath, 0400); err != nil {
		t.Fatalf("Unable to change log permissions: %v", err)
	}

	// Send an heartbeat with higher term, which should cause the current leader
	// to step down.
	sendAndWait(node, &pb.Heartbeat{Term: newTerm + 10, Leader: "other"})

	// Server should stepdown
	if state := waitForState(node, FOLLOWER); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got %s", state)
	}

	// Reset the permission
	os.Chmod(node.logPath, 0660)
}
