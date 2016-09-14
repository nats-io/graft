// Copyright 2013-2016 Apcera Inc. All rights reserved.

package graft

import (
	"os"
	"testing"
	"time"

	"github.com/nats-io/graft/pb"
)

// Dumb wait program to sync on callbacks, etc... Will timeout
func wait(t *testing.T, ch chan StateChange) *StateChange {
	select {
	case sc := <-ch:
		return &sc
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
