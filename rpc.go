// Copyright 2013 Apcera Inc. All rights reserved.

package graft

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
	SendVoteResponse(candidate string, vresp *VoteResponse) error
	// Used by Candidate Nodes to issue a new vote for a leader.
	RequestVote(*VoteRequest) error
	// Used by Leader Nodes to Heartbeat
	HeartBeat(*Heartbeat) error
}

// VoteRequest
type VoteRequest struct {
	// Term for the candidate.
	Term uint64

	// The candidate for the election.
	Candidate string
}

// VoteResponse
type VoteResponse struct {
	// The responder's term.
	Term uint64

	// Vote's status
	Granted bool
}

// Heartbeat
type Heartbeat struct {
	// Leader's current term.
	Term uint64

	// Leaders id.
	Leader string
}
