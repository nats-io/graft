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
	"testing"
	"time"

	"github.com/nats-io/gnatsd/test"
	"github.com/nats-io/go-nats"
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
	s := test.RunServer(&test.DefaultTestOptions)
	defer s.Shutdown()

	toStart := 5
	nodes := createNatsNodes(t, "nats_test", toStart)

	// Do cleanup
	for _, n := range nodes {
		defer n.Close()
	}

	// Wait for Election
	expectedClusterState(t, nodes, 1, toStart-1, 0)

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
	expectedClusterState(t, nodes, 1, toStart-2, 0)
}
