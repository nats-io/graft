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
