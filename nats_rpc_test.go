// Copyright 2013-2016 Apcera Inc. All rights reserved.

package graft

import (
	"fmt"
	"testing"
	"time"

	"github.com/nats-io/gnatsd/test"
	"github.com/nats-io/nats"
)

func createNatsNodes(t *testing.T, name string, numNodes int) []*Node {
	opts := &nats.DefaultOptions
	url := fmt.Sprintf("nats://%s:%d/",
		test.DefaultTestOptions.Host,
		test.DefaultTestOptions.Port)
	opts.Url = url

	ci := ClusterInfo{Name: name, Size: numNodes}
	nodes := make([]*Node, numNodes)
	for i := 0; i < numNodes; i++ {
		hand, _, logPath := genNodeArgs(t)
		rpc, err := NewNatsRpc(opts)
		if err != nil {
			t.Fatalf("NatsRPC error: %v", err)
		}
		node, err := New(ci, hand, rpc, logPath)
		if err != nil {
			t.Fatalf("Expected no error, got: %v", err)
		}
		nodes[i] = node
	}
	return nodes
}

func TestNatsLeaderElection(t *testing.T) {
	test.RunServer(&test.DefaultTestOptions)

	toStart := 5
	nodes := createNatsNodes(t, "nats_test", toStart)

	// Do cleanup
	for _, n := range nodes {
		defer n.Close()
	}

	// Wait for Election
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

	// Make sure he stays the leader (heartbeat functionality)
	leader := findLeader(nodes)

	// Wait for Election timout
	time.Sleep(MAX_ELECTION_TIMEOUT)

	if newLeader := findLeader(nodes); newLeader != leader {
		t.Fatalf("Expected leader to keep power, was %q, now %q\n",
			leader.Id(), newLeader.Id())
	}

	// Now close the leader, make sure someone else gets elected
	leader.Close()

	// Wait for Election
	time.Sleep(MAX_ELECTION_TIMEOUT)

	leaders, followers, candidates = countTypes(nodes)

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
