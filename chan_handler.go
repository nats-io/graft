// Copyright 2013 Apcera Inc. All rights reserved.

package graft

// ChanHandler is a convenience handler when a user wants to simply use
// channels for the async handling of errors and state changes.
type ChanHandler struct {
	// Chan to receive state changes.
	StateChangeChan chan<- StateChange
	// Chan to receive errors.
	ErrorChan chan<- error
}

// StateChange captures to from and to States for the ChanHandler.
type StateChange struct {
	from, to State
}

func NewChanHandler(scCh chan<- StateChange, errCh chan<- error) *ChanHandler {
	return &ChanHandler{scCh, errCh}
}

// Queue the state change onto the channel
func (chand *ChanHandler) StateChange(from, to State) {
	chand.StateChangeChan <- StateChange{from, to}
}

// Queue the error onto the channel
func (chand *ChanHandler) AsyncError(err error) {
	chand.ErrorChan <- err
}


