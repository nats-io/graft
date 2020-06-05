// Copyright 2013-2020 The NATS Authors
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
	ErrClusterName  = errors.New("graft: Cluster name can not be empty")
	ErrClusterSize  = errors.New("graft: Cluster size can not be 0")
	ErrHandlerReq   = errors.New("graft: Handler is required")
	ErrRpcDriverReq = errors.New("graft: RPCDriver is required")
	ErrLogReq       = errors.New("graft: Log is required")
	ErrLogNoExist   = errors.New("graft: Log file does not exist")
	ErrLogNoState   = errors.New("graft: Log file does not have any state")
	ErrLogCorrupt   = errors.New("graft: Encountered corrupt log file")
	ErrNotImpl      = errors.New("graft: Not implemented")
)
