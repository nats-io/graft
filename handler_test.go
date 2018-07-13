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
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/nedscode/graft/pb"
)

// Dumb wait program to sync on callbacks, etc... Will timeout
func wait(t *testing.T, ch chan StateChange) *StateChange {
	select {
	case sc := <-ch:
		return &sc
	// It could be that the random election time is the max. Inc
	// that case, the node will still need a bit more time to
	// transition.
	case <-time.After(MAX_ELECTION_TIMEOUT + 50*time.Millisecond):
		t.Fatal("Timeout waiting on state change")
	}
	return nil
}

func errWait(t *testing.T, ch chan error) error {
	select {
	case err := <-ch:
		return err
	case <-time.After(MAX_ELECTION_TIMEOUT):
		t.Fatal("Timeout waiting on error handler")
	}
	return nil
}

func TestStateChangeHandler(t *testing.T) {
	ci := ClusterInfo{Name: "foo", Size: 1}
	_, rpc, log := genNodeArgs(t)

	// Use ChanHandler
	scCh := make(chan StateChange)
	errCh := make(chan error)
	chHand := NewChanHandler(scCh, errCh)

	node, err := New(ci, chHand, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	defer node.Close()

	sc := wait(t, scCh)
	if sc.From != FOLLOWER && sc.To != CANDIDATE {
		t.Fatalf("Did not receive correct states for state change: %+v\n", sc)
	}
	sc = wait(t, scCh)
	if sc.From != CANDIDATE && sc.To != LEADER {
		t.Fatalf("Did not receive correct states for state change: %+v\n", sc)
	}

	// Force the leader to stepdown by using a larger term.
	newTerm := node.CurrentTerm() + 1
	node.HeartBeats <- &pb.Heartbeat{Term: newTerm, Leader: "new"}

	sc = wait(t, scCh)
	if sc.From != LEADER && sc.To != FOLLOWER {
		t.Fatalf("Did not receive correct states for state change: %+v\n", sc)
	}
}

// The only real errors right now are log based or RPC.
func TestErrorHandler(t *testing.T) {
	ci := ClusterInfo{Name: "foo", Size: 1}
	_, rpc, log := genNodeArgs(t)

	// Use ChanHandler
	scCh := make(chan StateChange)
	errCh := make(chan error)
	chHand := NewChanHandler(scCh, errCh)

	node, err := New(ci, chHand, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	defer node.Close()

	// Force a write to err
	os.Chmod(node.logPath, 0400)
	defer os.Chmod(node.logPath, 0660)

	err = errWait(t, errCh)

	perr, ok := err.(*os.PathError)
	if !ok {
		t.Fatalf("Got wrong error type")
	}
	if perr.Op != "open" {
		t.Fatalf("Got wrong operation, wanted 'open', got %q", perr.Op)
	}
	if perr.Path != node.LogPath() {
		t.Fatalf("Expected the logPath, got %s \n", perr.Path)
	}
}

func TestChandHandlerNotBlockingNode(t *testing.T) {
	ci := ClusterInfo{Name: "foo", Size: 1}
	_, rpc, log := genNodeArgs(t)

	// Use ChanHandler
	scCh := make(chan StateChange)
	errCh := make(chan error)
	chHand := NewChanHandler(scCh, errCh)

	node, err := New(ci, chHand, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	defer node.Close()

	// Reset the node election timer to a very high number so
	// it does not interfere with the test
	node.mu.Lock()
	node.electTimer.Reset(time.Hour)
	node.mu.Unlock()
	// Wait in case election was happening
	time.Sleep(MAX_ELECTION_TIMEOUT + 50*time.Millisecond)
	// Drain the state changes
	drained := false
	for !drained {
		select {
		case <-scCh:
		default:
			drained = true
		}
	}

	// Make sure that if no one is dequeuing state changes or errors
	// the node is not blocked.
	total := 10
	to := FOLLOWER
	step := 0
	for i := 0; i < total; i++ {
		switch step {
		case 0:
			to = CANDIDATE
			step = 1
		case 1:
			to = LEADER
			step = 2
		case 2:
			to = FOLLOWER
			step = 0
		}
		// This call expects lock to be held on entry
		node.mu.Lock()
		node.switchState(to)
		node.mu.Unlock()
		// This call does not
		node.handleError(fmt.Errorf("%d", i))
	}

	// Now dequeue from channels and verify order
	step = 0
	for i := 0; i < total; i++ {
		sc := <-scCh
		switch step {
		case 0:
			if sc.From != FOLLOWER || sc.To != CANDIDATE {
				t.Fatalf("i=%d Expected state to be from Follower to Candidate, got %v to %v", i, sc.From.String(), sc.To.String())
			}
			step = 1
		case 1:
			if sc.From != CANDIDATE || sc.To != LEADER {
				t.Fatalf("i=%d Expected state to be from Candidate to Leader, got %v to %v", i, sc.From.String(), sc.To.String())
			}
			step = 2
		case 2:
			if sc.From != LEADER || sc.To != FOLLOWER {
				t.Fatalf("i=%d Expected state to be from Leader to Follower, got %v to %v", i, sc.From.String(), sc.To.String())
			}
			step = 0
		}
		err := <-errCh
		errAsInt, convErr := strconv.Atoi(err.Error())
		if convErr != nil {
			t.Fatalf("Error converting error content: %v", convErr)
		}
		if errAsInt != i {
			t.Fatalf("Expected error to be %d, got %d", i, errAsInt)
		}
	}
}
