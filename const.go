// Copyright 2013 Apcera Inc. All rights reserved.

package graft

import (
	"time"
)

const (
	VERSION = "0.1"

	// Election timout MIN and MAX
	MIN_ELECTION_TIMEOUT = 500 * time.Millisecond
	MAX_ELECTION_TIMEOUT = 2 * MIN_ELECTION_TIMEOUT

	// Heartbeat tick for LEADERS. Should be << MIN_ELECTION_TIMEOUT
	HEARTBEAT_INTERVAL = 100 * time.Millisecond

	NO_LEADER = ""
	NO_VOTE   = ""

	// Use buffer channels.
	CHAN_SIZE = 8
)
