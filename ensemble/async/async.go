/*
 * Copyright The Microcks Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *  http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */
package async

import (
	"context"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	DefaultImage = "quay.io/microcks/microcks-uber-async-minion:latest"

	// DefaultHttpPort represents the default Microcks Async Minion HTTP port
	DefaultHttpPort = "8081/tcp"

	// DefaultNetworkAlias represents the default network alias of the the PostmanContainer
	DefaultNetworkAlias = "microcks-async-minion"
)

// MicrocksAysncMinionContainer represents the Microcks Async Minion container type used in the module.
type MicrocksAysncMinionContainer struct {
	testcontainers.Container
}

// RunContainer creates an instance of the MicrocksAysncMinionContainer type.
func RunContainer(ctx context.Context, microcksHostPort string, opts ...testcontainers.ContainerCustomizer) (*MicrocksAysncMinionContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        DefaultImage,
		ExposedPorts: []string{DefaultHttpPort},
		WaitingFor:   wait.ForLog("Profile prod activated"),
		Env:          map[string]string{"MICROCKS_HOST_PORT": microcksHostPort},
	}
	genericContainerReq := testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	}

	for _, opt := range opts {
		opt.Customize(&genericContainerReq)
	}

	container, err := testcontainers.GenericContainer(ctx, genericContainerReq)
	if err != nil {
		return nil, err
	}

	return &MicrocksAysncMinionContainer{Container: container}, nil
}

// WithNetwork allows to add a custom network
func WithNetwork(networkName string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Networks = append(req.Networks, networkName)
	}
}

// WithNetworkAlias allows to add a custom network alias for a specific network
func WithNetworkAlias(networkName, networkAlias string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		if req.NetworkAliases == nil {
			req.NetworkAliases = make(map[string][]string)
		}
		req.NetworkAliases[networkName] = []string{networkAlias}
	}
}
