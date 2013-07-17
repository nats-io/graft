Graft
=====

A RAFT Election implementation in Go.

Overview
=====

RAFT is a consensus based algorithm that produces consistent state through replicated logs and leader elections. Continuum has several uses for an election algorithm that produces guaranteed leaders for N-wise scalability and elimination of SPOF (Single Point of Failure). In the current design, both the Health Manager and the AuthServer will utilize an elected leader.

