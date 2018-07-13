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
	"github.com/nedscode/graft/pb"
)

// An RPCDriver allows multiple transports to be utilized for the
// RAFT protocol RPCs. An instance of RPCDriver will use the *Node
// passed to Init() to call back into the Node when VoteRequests,
// VoteResponses and Heartbeat RPCs are received. They will be
// placed on the appropriate node's channels.
type RPCDriver interface {
	// Used to initialize the driver
	Init(*Node) error
	// Used to close down any state
	Close()
	// Used to respond to VoteResponses to candidates
	SendVoteResponse(candidate string, vresp *pb.VoteResponse) error
	// Used by Candidate Nodes to issue a new vote for a leader.
	RequestVote(*pb.VoteRequest) error
	// Used by Leader Nodes to Heartbeat
	HeartBeat(*pb.Heartbeat) error
}
