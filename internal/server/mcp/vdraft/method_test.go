// Copyright 2026 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package vdraft

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/googleapis/mcp-toolbox/internal/log"
	"github.com/googleapis/mcp-toolbox/internal/server/mcp/jsonrpc"
	"github.com/googleapis/mcp-toolbox/internal/server/resources"
	"github.com/googleapis/mcp-toolbox/internal/testutils"
	"github.com/googleapis/mcp-toolbox/internal/tools"
	"github.com/googleapis/mcp-toolbox/internal/util"
	"github.com/googleapis/mcp-toolbox/internal/util/parameters"
)

var (
	dummyID           jsonrpc.RequestId = 1
	fakeVersionString                   = "0.0.0"
)

func TestToolsCallHandlerWithSecureParams(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	testLogger, err := log.NewStdLogger(os.Stdout, os.Stderr, "info")
	if err != nil {
		t.Fatalf("unable to initialize logger: %s", err)
	}
	ctxLogger := util.WithLogger(ctx, testLogger)
	ctxVersion := util.WithToolboxVersionKey(ctxLogger, fakeVersionString)

	secureTool := testutils.NewMockTool(
		"secure_tool",
		"A tool with secure parameters",
		parameters.Parameters{
			&parameters.StringParameter{
				CommonParameter: parameters.CommonParameter{
					Name:   "api_key",
					Type:   parameters.TypeString,
					Desc:   "A secure api key",
					Secure: true,
				},
			},
			parameters.NewStringParameter("query", "A standard search query"),
		},
		false,
		false,
	)

	toolsMap := map[string]tools.Tool{
		"secure_tool": secureTool,
	}

	tc := tools.ToolsetConfig{
		Name:      "test-toolset",
		ToolNames: []string{"secure_tool"},
	}
	toolset, err := tc.Initialize("test-version", toolsMap)
	if err != nil {
		t.Fatalf("failed initializing toolset: %s", err)
	}

	toolsets := map[string]tools.Toolset{"test-toolset": toolset}
	resourceMgr := resources.NewResourceManager(nil, nil, nil, toolsMap, toolsets, nil, nil)

	tests := []struct {
		desc        string
		body        string // raw JSON-RPC body
		wantErr     bool
		errContains string
	}{
		{
			desc: "Client does not support secure parameters",
			body: `{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "tools/call",
				"params": {
					"name": "secure_tool",
					"arguments": {
						"query": "hello"
					},
					"secureArguments": {
						"api_key": "secret"
					}
				}
			}`,
			wantErr:     true,
			errContains: "requires secure-params extension which is not supported by the client",
		},
		{
			desc: "Secure parameter passed in standard arguments",
			body: `{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "tools/call",
				"params": {
					"name": "secure_tool",
					"arguments": {
						"query": "hello",
						"api_key": "secret"
					},
					"_meta": {
						"capabilities": {
							"toolbox/secure-params": true
						}
					}
				}
			}`,
			wantErr:     true,
			errContains: "parameter \"api_key\" is secure and must not be passed in standard arguments",
		},
		{
			desc: "Standard parameter passed in secureArguments",
			body: `{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "tools/call",
				"params": {
					"name": "secure_tool",
					"arguments": {},
					"secureArguments": {
						"query": "hello",
						"api_key": "secret"
					},
					"_meta": {
						"capabilities": {
							"toolbox/secure-params": true
						}
					}
				}
			}`,
			wantErr:     true,
			errContains: "parameter \"query\" is not secure and must not be passed in secureArguments",
		},
		{
			desc: "Successful invocation with correct routing",
			body: `{
				"jsonrpc": "2.0",
				"id": 1,
				"method": "tools/call",
				"params": {
					"name": "secure_tool",
					"arguments": {
						"query": "hello"
					},
					"secureArguments": {
						"api_key": "secret"
					},
					"_meta": {
						"capabilities": {
							"toolbox/secure-params": true
						}
					}
				}
			}`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.desc, func(t *testing.T) {
			got, err := toolsCallHandler(ctxVersion, dummyID, toolset, resourceMgr, []byte(tt.body), nil)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errContains != "" && !strings.Contains(err.Error(), tt.errContains) {
					t.Errorf("error = %v, want string containing %q", err, tt.errContains)
				}
			} else {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				if got == nil {
					t.Errorf("expected valid response, got nil")
				}
			}
		})
	}
}
