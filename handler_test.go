// Copyright 2013 Apcera Inc. All rights reserved.

package graft

import (
"fmt"

	"os"
	"testing"
	"time"
)

type stateChange struct {
	from, to State
}

type mockHandler struct {
	scChan  chan *stateChange
	errChan chan error
}

func (mh *mockHandler) AsyncError(err error) {
	mh.errChan <- err
}

func (mh *mockHandler) StateChange(from, to State) {
	mh.scChan <- &stateChange{from, to}
}

// Dumb wait program to sync on callbacks, etc... Will timeout
func wait(t *testing.T, ch chan *stateChange) *stateChange {
	select {
	case sc := <-ch:
		return sc
	case <-time.After(MAX_ELECTION_TIMEOUT):
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

	// Use mockHandler
	mh := &mockHandler{make(chan *stateChange), make(chan error)}

	node, err := New(ci, mh, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	defer node.Close()

	sc := wait(t, mh.scChan)
	if sc.from != FOLLOWER && sc.to != CANDIDATE {
		t.Fatalf("Did not receive correct states for state change: %+v\n", sc)
	}
	sc = wait(t, mh.scChan)
	if sc.from != CANDIDATE && sc.to != LEADER {
		t.Fatalf("Did not receive correct states for state change: %+v\n", sc)
	}

	// Force the leader to stepdown.
	node.HeartBeats <- &Heartbeat{Term: 20, Leader: "new"}

	sc = wait(t, mh.scChan)
	if sc.from != LEADER && sc.to != FOLLOWER {
		t.Fatalf("Did not receive correct states for state change: %+v\n", sc)
	}
}

// The only real errors right now are log based or RPC.
func TestErrorHandler(t *testing.T) {
	ci := ClusterInfo{Name: "foo", Size: 1}
	_, rpc, log := genNodeArgs(t)

	// Use mockHandler
	mh := &mockHandler{make(chan *stateChange), make(chan error)}

	node, err := New(ci, mh, rpc, log)
	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	defer node.Close()

	// Force a write to err
	os.Chmod(node.logPath, 0400)
	defer os.Chmod(node.logPath, 0660)

	err = errWait(t, mh.errChan)

	perr, ok := err.(*os.PathError)
	if !ok {
		t.Fatalf("Got wrong error type")
	}
	if perr.Op != "open" {
		t.Fatalf("Got wrong operation, wanted 'open', got %q", perr.Op)
	}
	if perr.Path != node.logPath {
		t.Fatalf("Expected the logPath, got %\n", perr.Path)
	}
}
