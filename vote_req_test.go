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
	"encoding/binary"
	"os"
	"testing"
	"time"

	"github.com/nedscode/graft/pb"
)

// Test VoteRequests RPC in different states.

type stateMachineHandler struct {
	logIndex uint32
}

func (s *stateMachineHandler) CurrentState() []byte {
	buf := make([]byte, 4)
	binary.BigEndian.PutUint32(buf, s.logIndex)
	return buf
}

func (s *stateMachineHandler) GrantVote(position []byte) bool {
	return binary.BigEndian.Uint32(position) >= s.logIndex
}

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
	fake.VoteResponses = make(chan *pb.VoteResponse)
	fake.VoteRequests = make(chan *pb.VoteRequest, 32)
	return fake
}

func TestQuorum(t *testing.T) {
	type test struct{ cluster, quorum int }
	tests := []test{{0, 0}, {1, 1}, {2, 2}, {3, 2}, {4, 3}, {9, 5}, {12, 7}}
	for _, tc := range tests {
		if q := quorumNeeded(tc.cluster); q != tc.quorum {
			t.Fatalf("Expected quorum size of %d with cluster size %d, got %d\n",
				tc.quorum, tc.cluster, q)
		}
	}
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
	node.VoteRequests <- &pb.VoteRequest{Term: 1, Candidate: fake.id}
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

	node.VoteRequests <- &pb.VoteRequest{Term: newTerm, Candidate: fake.id}
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

func TestVoteRequestAsFollowerLogBehind(t *testing.T) {
	ci := ClusterInfo{Name: "vreq", Size: 3}
	_, rpc, log := genNodeArgs(t)
	stateHandler := new(stateMachineHandler)
	scCh := make(chan StateChange)
	errCh := make(chan error)
	handler := NewChanHandlerWithStateMachine(stateHandler, scCh, errCh)
	node, err := New(ci, handler, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	defer node.Close()

	// Set log position to artificial higher value.
	newPosition := uint32(8)
	term := uint64(1)
	stateHandler.logIndex = newPosition
	node.setTerm(term)

	// Force write of state
	node.writeState()

	// Create fake node to watch VoteResponses.
	fake := fakeNode("fake")

	// Hook up to MockRPC layer
	mockRegisterPeer(fake)
	defer mockUnregisterPeer(fake.id)

	// a VoteRequest with a log that is behind should be ignored
	pos := make([]byte, 4)
	binary.BigEndian.PutUint32(pos, 1)
	node.VoteRequests <- &pb.VoteRequest{Term: term, Candidate: fake.id, CurrentState: pos}
	vresp := <-fake.VoteResponses
	if vresp.Term != term {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			term, vresp.Term)
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
	if node.CurrentTerm() != term {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			term, node.CurrentTerm())
	}

	// Test persistent state
	testStateOfNode(t, node)

	// a VoteRequest with a log that is ahead should reset follower
	binary.BigEndian.PutUint32(pos, newPosition+1)
	node.VoteRequests <- &pb.VoteRequest{Term: term, Candidate: fake.id, CurrentState: pos}
	vresp = <-fake.VoteResponses
	if vresp.Term != term {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			term, vresp.Term)
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
	if node.CurrentTerm() != term {
		t.Fatalf("Expected CurrentTerm of %d, got: %d\n",
			term, node.CurrentTerm())
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
		t.Fatalf("Expected Node to have a cast vote for itself, got:%s", vote)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// a VoteRequest with lower term should be ignored
	node.VoteRequests <- &pb.VoteRequest{Term: 1, Candidate: fake.id}
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
		t.Fatalf("Expected Node to have still cast vote for itself, got:%s", vote)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// a VoteRequest for same term but different candidate should be
	// denied since we always vote for ourself.
	node.VoteRequests <- &pb.VoteRequest{Term: newTerm, Candidate: fake.id}
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
		t.Fatalf("Expected Node to have still cast vote for itself, got:%s", vote)
	}

	// Test persistent state
	testStateOfNode(t, node)

	// a VoteRequest with a higher term should reset to follower
	newTerm++
	node.VoteRequests <- &pb.VoteRequest{Term: newTerm, Candidate: fake.id}
	vresp = <-fake.VoteResponses
	if vresp.Term != newTerm {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			newTerm, vresp.Term)
	}
	if vresp.Granted == false {
		t.Fatal("Expected the VoteResponse to have been Granted")
	}
	// Verify new state
	if state := waitForState(node, FOLLOWER); state != FOLLOWER {
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
	node.VoteResponses <- &pb.VoteResponse{Term: vreq.Term, Granted: true}

	if state := waitForState(node, LEADER); state != LEADER {
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
	node.VoteRequests <- &pb.VoteRequest{Term: 1, Candidate: fake.id}
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

	// a VoteRequest with same term should be ignored
	node.VoteRequests <- &pb.VoteRequest{Term: newTerm, Candidate: fake.id}
	vresp = <-fake.VoteResponses
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
	node.VoteRequests <- &pb.VoteRequest{Term: newTerm, Candidate: fake.id}
	vresp = <-fake.VoteResponses
	if vresp.Term != newTerm {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			newTerm, vresp.Term)
	}
	if vresp.Granted == false {
		t.Fatal("Expected the VoteResponse to have been Granted")
	}
	// Verify new state
	if state := waitForState(node, FOLLOWER); state != FOLLOWER {
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

func TestVoteRequestAsLeaderStepsDownDueToErrorOnSave(t *testing.T) {
	leader := vreqNode(t, 1)
	defer leader.Close()

	leader.mu.Lock()
	leader.electTimer.Reset(time.Millisecond)
	leader.mu.Unlock()

	// Wait for this node to be elected leader
	waitForLeader(leader, leader.Id())

	// Change log permissions to cause error
	if err := os.Chmod(leader.logPath, 0400); err != nil {
		t.Fatalf("Unable to change log permissions: %v", err)
	}
	defer os.Chmod(leader.logPath, 0660)

	// Create fake node to watch VoteResponses.
	fake := fakeNode("fake")

	// Hook up to MockRPC layer
	mockRegisterPeer(fake)
	defer mockUnregisterPeer(fake.id)

	leader.VoteRequests <- &pb.VoteRequest{Candidate: fake.Id(), Term: 2}
	vresp := <-fake.VoteResponses
	if vresp.Term != 1 {
		t.Fatalf("Expected the VoteResponse to have term=%d, got %d\n",
			1, vresp.Term)
	}
	if vresp.Granted != false {
		t.Fatal("Expected the VoteResponse to have Granted of false")
	}
	// Restore permissions
	os.Chmod(leader.logPath, 0660)

	// The leader should step down
	if state := waitForState(leader, FOLLOWER); state != FOLLOWER {
		t.Fatalf("Expected Node to be in Follower state, got: %s", state)
	}
}
