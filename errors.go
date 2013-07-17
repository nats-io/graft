// Copyright 2013 Apcera Inc. All rights reserved.

package graft

import (
	"errors"
)

var (
	ClusterNameErr  = errors.New("grafty: Cluster name can not be empty")
	ClusterSizeErr  = errors.New("grafty: Cluster size can not be 0")
	HandlerReqErr   = errors.New("grafty: Handler is required")
	RpcDriverReqErr = errors.New("grafty: RPCDriver is required")
	LogReqErr       = errors.New("grafty: Log is required")
	LogNoExistErr   = errors.New("grafty: Log file does not exist")
	LogNoStateErr   = errors.New("grafty: Log file does not have any state")
	LogCorruptErr   = errors.New("grafty: Encountered corrupt log file")
	NotImplErr      = errors.New("grafty: Not implemented")
)
