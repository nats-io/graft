Graft
=====

A RAFT Election implementation in Go. More information on RAFT can be found in this [research paper](https://ramcloud.stanford.edu/wiki/download/attachments/11370504/raft.pdf
) and this [video](http://www.youtube.com/watch?v=YbZ3zDzDnrw&list=WL20FE97C942825E1E).

[![Build Status](https://magnum.travis-ci.com/apcera/graft.png?token=UGjrGa8sFWGQcHSJeAvp)](https://magnum.travis-ci.com/apcera/graft)

Overview
=====

RAFT is a consensus based algorithm that produces consistent state through replicated logs and leader elections.
Continuum has several uses for an election algorithm that produces guaranteed leaders for N-wise scalability and
elimination of SPOF (Single Point of Failure). In the current design, both the Health Manager and the AuthServer
will utilize an elected leader.

## Example Usage

```go

ci := graft.ClusterInfo{Name: "health_manager", Size: 3}
rpc, err := graft.NewNatsRpc(&nats.DefaultOptions)
handler := graft.NewChanHandler(make(chan StateChange), make(chan error))

node, err := graft.New(ci, handler, rpc, "/tmp/graft.log");

// ...

if node.State() == graft.LEADER {
  // Process as a LEADER
}

select {
  case sc := <- stateChangeChan:
    // Process a state change
  case err := <- errChan:
    // Process an error, log etc.
}

```
