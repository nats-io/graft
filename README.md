Graft
=====

A RAFT Election implementation in Go. More information on RAFT can be found in this [research paper](https://ramcloud.stanford.edu/wiki/download/attachments/11370504/raft.pdf
) and this [video](http://www.youtube.com/watch?v=YbZ3zDzDnrw&list=WL20FE97C942825E1E).

[![License Apache 2](https://img.shields.io/badge/License-Apache2-blue.svg)](https://www.apache.org/licenses/LICENSE-2.0)
[![Build Status](https://travis-ci.org/nats-io/graft.svg?branch=master)](http://travis-ci.org/nats-io/graft)
[![Coverage Status](https://coveralls.io/repos/github/nats-io/graft/badge.svg)](https://coveralls.io/github/nats-io/graft)

Overview
=====

RAFT is a consensus based algorithm that produces consistent state through replicated logs and leader elections.

Example usage of the election algorithm is to produce guaranteed leaders for N-wise scalability and elimination
of SPOF (Single Point of Failure) within a system.

## Example Usage

```go

ci := graft.ClusterInfo{Name: "health_manager", Size: 3}
rpc, err := graft.NewNatsRpc(&nats.DefaultOptions)
errChan := make(chan error)
stateChangeChan := make(chan StateChange)
handler := graft.NewChanHandler(stateChangeChan, errChan)

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

## License

Unless otherwise noted, the NATS source files are distributed
under the Apache Version 2.0 license found in the LICENSE file.
