// Copyright 2013 Apcera Inc. All rights reserved.

package graft

import (
	"errors"
	"fmt"

	"github.com/apcera/nats"
)

// The subject space for the nats rpc driver is based on the
// cluster name, which is filled in below on the heartbeats
// and vote requests. The vote responses are directed by
// using the node.Id().
const (
	HEARTBEAT_SUB = "grafty.%s.heartbeat"
	VOTE_REQ_SUB  = "grafty.%s.vote_request"
	VOTE_RESP_SUB = "grafty.%s.vote_response"
)

var (
	NotInitializedErr = errors.New("grafty(nats_rpc): Driver is not properly initialized")
)

type NatsRpcDriver struct {
	// NATS encoded connection.
	ec *nats.EncodedConn

	// Heartbeat subscription.
	hbSub *nats.Subscription

	// Vote request subscription.
	vreqSub *nats.Subscription

	// Vote response subscription.
	vrespSub *nats.Subscription

	// Graft node.
	node *Node
}

// Create a new instance of the driver. The NATS encoded connection
// will use json and the options passed in.
func NewNatsRpc(opts *nats.Options) (*NatsRpcDriver, error) {
	nc, err := opts.Connect()
	ec, _ := nats.NewEncodedConn(nc, "json")
	if err != nil {
		return nil, err
	}
	return &NatsRpcDriver{ec: ec}, nil
}

// Initialize via the Graft node.
func (rpc *NatsRpcDriver) Init(n *Node) (err error) {
	rpc.node = n

	// Create the heartbeat subscription.
	hbSub := fmt.Sprintf(HEARTBEAT_SUB, n.ClusterInfo().Name)
	rpc.hbSub, err = rpc.ec.Subscribe(hbSub, rpc.HeartbeatCallback)
	if err != nil {
		return err
	}
	// Create the voteRequest subscription.
	rpc.vreqSub, err = rpc.ec.Subscribe(rpc.vreqSubject(), rpc.VoteRequestCallback)
	if err != nil {
		return err
	}
	return nil
}

// Close down the subscriptions and the NATS encoded connection.
// Will nil everything out.
func (rpc *NatsRpcDriver) Close() {
	if rpc.hbSub != nil {
		rpc.hbSub.Unsubscribe()
		rpc.hbSub = nil
	}
	if rpc.vreqSub != nil {
		rpc.vreqSub.Unsubscribe()
		rpc.vreqSub = nil
	}
	if rpc.vrespSub != nil {
		rpc.vrespSub.Unsubscribe()
		rpc.vrespSub = nil
	}
	if rpc.ec != nil {
		rpc.ec.Close()
		rpc.ec = nil
	}
}

// Convenience function for generating the directed response
// subject for vote requests. We will use the candidate's id
// to form a directed response
func (rpc *NatsRpcDriver) vrespSubject(candidate string) string {
	return fmt.Sprintf(VOTE_RESP_SUB, candidate)
}

// Convenience funstion for generating the vote request subject.
func (rpc *NatsRpcDriver) vreqSubject() string {
	return fmt.Sprintf(VOTE_REQ_SUB, rpc.node.ClusterInfo().Name)
}

// Heartbeat callback which will place the heartbeat on the Graft
// node's appropriate channel.
func (rpc *NatsRpcDriver) HeartbeatCallback(hb *Heartbeat) {
	rpc.node.HeartBeats <- hb
}

// VoteRequest callback which will place the request on the Graft
// node's appropriate channel.
func (rpc *NatsRpcDriver) VoteRequestCallback(vreq *VoteRequest) {
	// Don't respond to our own request.
	if vreq.Candidate != rpc.node.Id() {
		rpc.node.VoteRequests <- vreq
	}
}

// VoteResponse callback which will place the response on the Graft
// node's appropriate channel.
func (rpc *NatsRpcDriver) VoteResponseCallback(vresp *VoteResponse) {
	rpc.node.VoteResponses <- vresp
}

// RequestVote is sent from the Graft node when it has become a
// candidate.
func (rpc *NatsRpcDriver) RequestVote(vr *VoteRequest) error {
	// Create a new response subscription for each oustanding
	// RequestVote and cancel the previous.
	if rpc.vrespSub != nil {
		rpc.vrespSub.Unsubscribe()
		rpc.vrespSub = nil
	}
	inbox := rpc.vrespSubject(rpc.node.Id())
	sub, err := rpc.ec.Subscribe(inbox, rpc.VoteResponseCallback)
	if err != nil {
		return err
	}
	// If we can auto-unsubscribe to max number of expected responses
	// which will be the cluster size.
	if size := rpc.node.ClusterInfo().Size; size > 0 {
		sub.AutoUnsubscribe(size)
	}
	// hold to cancel later.
	rpc.vrespSub = sub
	// Fire off the request.
	return rpc.ec.PublishRequest(rpc.vreqSubject(), inbox, vr)
}

// Heartbeat is called from the Graft node to send out a heartbeat
// while it is a LEADER.
func (rpc *NatsRpcDriver) HeartBeat(hb *Heartbeat) error {
	if rpc.hbSub == nil {
		return NotInitializedErr
	}
	return rpc.ec.Publish(rpc.hbSub.Subject, hb)
}

// SendVoteResponse is called from the Graft node to responsd to a vote request.
func (rpc *NatsRpcDriver) SendVoteResponse(id string, vresp *VoteResponse) error {
	return rpc.ec.Publish(rpc.vrespSubject(id), vresp)
}
