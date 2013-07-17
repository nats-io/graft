// Copyright 2013 Apcera Inc. All rights reserved.

package graft

import (
	"fmt"
	"time"
)

const (
	VERSION = "0.1"

	// Election timout MIN and MAX
	MIN_ELECTION_TIMEOUT = 500 * time.Millisecond
	MAX_ELECTION_TIMEOUT = 2 * MIN_ELECTION_TIMEOUT

	// Heartbeat tick for LEADERS. Should be << MIN_ELECTION
	HEARTBEAT_INTERVAL = 100 * time.Millisecond

	NO_LEADER = ""
	NO_VOTE   = ""

	// Use buffer channels.
	CHAN_SIZE = 8
)

type State int8

// Allowable states for a Graft node.
const (
	FOLLOWER State = iota
	LEADER
	CANDIDATE
	CLOSED
)

// Convenience for printing, etc.
func (s State) String() string {
	switch s {
	case FOLLOWER:
		return "Follower"
	case LEADER:
		return "Leader"
	case CANDIDATE:
		return "Candidate"
	case CLOSED:
		return "Closed"
	default:
		return fmt.Sprintf("Unknown[%d]", s)
	}
}
