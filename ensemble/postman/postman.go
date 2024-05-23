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
package postman

import (
	"context"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultImage = "quay.io/microcks/microcks-postman-runtime:latest"

	// DefaultHTTPPort represents the default Postman HTTP port
	DefaultHTTPPort = "3000/tcp"

	// DefaultNetworkAlias represents the default network alias of the the PostmanContainer
	DefaultNetworkAlias = "postman"
)

// PostmanContainer represents the Postman container type used in the ensemble.
type PostmanContainer struct {
	testcontainers.Container
}

// RunContainer runs the Postman container
func RunContainer(ctx context.Context, opts ...testcontainers.ContainerCustomizer) (*PostmanContainer, error) {
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        defaultImage,
			ExposedPorts: []string{DefaultHTTPPort},
			WaitingFor:   wait.ForLog("Microcks postman-runtime wrapper listening on port: 3000"),
		},
		Started: true,
	}

	for _, opt := range opts {
		opt.Customize(&req)
	}

	container, err := testcontainers.GenericContainer(ctx, req)
	if err != nil {
		return nil, err
	}

	return &PostmanContainer{Container: container}, nil
}

// WithNetwork allows to add a custom network
func WithNetwork(networkName string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Networks = append(req.Networks, networkName)

		return nil
	}
}

// WithNetworkAlias allows to add a custom network alias for a specific network
func WithNetworkAlias(networkName, networkAlias string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.NetworkAliases == nil {
			req.NetworkAliases = make(map[string][]string)
		}
		req.NetworkAliases[networkName] = []string{networkAlias}

		return nil
	}
}
