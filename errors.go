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
	"errors"
)

var (
	ClusterNameErr  = errors.New("graft: Cluster name can not be empty")
	ClusterSizeErr  = errors.New("graft: Cluster size can not be 0")
	HandlerReqErr   = errors.New("graft: Handler is required")
	RpcDriverReqErr = errors.New("graft: RPCDriver is required")
	LogReqErr       = errors.New("graft: Log is required")
	LogNoExistErr   = errors.New("graft: Log file does not exist")
	LogNoStateErr   = errors.New("graft: Log file does not have any state")
	LogCorruptErr   = errors.New("graft: Encountered corrupt log file")
	NotImplErr      = errors.New("graft: Not implemented")
)
