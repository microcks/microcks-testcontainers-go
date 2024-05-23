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
	"fmt"
	"strconv"
	"strings"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"microcks.io/testcontainers-go/ensemble/async/connection/kafka"
)

const (
	DefaultImage = "quay.io/microcks/microcks-uber-async-minion:latest"

	// DefaultHttpPort represents the default Microcks Async Minion HTTP port
	DefaultHttpPort = "8081/tcp"

	// DefaultNetworkAlias represents the default network alias of the the PostmanContainer
	DefaultNetworkAlias = "microcks-async-minion"
)

// Option represents an option to pass to the minion
type Option func(*MicrocksAysncMinionContainer) error

// ContainerOptions represents the container options
type ContainerOptions struct {
	list []testcontainers.ContainerCustomizer
}

// Add adds an option to the list
func (co *ContainerOptions) Add(opt testcontainers.ContainerCustomizer) {
	co.list = append(co.list, opt)
}

// MicrocksAysncMinionContainer represents the Microcks Async Minion container type used in the module.
type MicrocksAysncMinionContainer struct {
	testcontainers.Container

	extraProtocols string

	containerOptions ContainerOptions
}

// RunContainer creates an instance of the MicrocksAysncMinionContainer type.
func RunContainer(ctx context.Context, microcksHostPort string, opts ...testcontainers.ContainerCustomizer) (*MicrocksAysncMinionContainer, error) {
	hostAccessPort, err := convertPortToInt(microcksHostPort)
	if err != nil {
		return nil, err
	}

	req := testcontainers.ContainerRequest{
		Image:           DefaultImage,
		ExposedPorts:    []string{DefaultHttpPort},
		HostAccessPorts: []int{hostAccessPort},
		WaitingFor:      wait.ForLog("Profile prod activated"),
		Env:             map[string]string{"MICROCKS_HOST_PORT": microcksHostPort},
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

// WithEnv allows to add an environment variable
func WithEnv(key, value string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.Env == nil {
			req.Env = make(map[string]string)
		}
		req.Env[key] = value

		return nil
	}
}

// WithKafkaConnection connects the MicrocksAsyncMinionContainer to a Kafka server to allow Kafka messages mocking.
func WithKafkaConnection(connection kafka.Connection) Option {
	return func(minion *MicrocksAysncMinionContainer) error {
		if !strings.Contains(minion.extraProtocols, ",KAFKA") {
			minion.extraProtocols = strings.Join([]string{minion.extraProtocols, ",KAFKA"}, "")
		}

		minion.containerOptions.Add(WithEnv("ASYNC_PROTOCOLS", minion.extraProtocols))
		minion.containerOptions.Add(WithEnv("KAFKA_BOOTSTRAP_SERVER", connection.BootstrapServers))
		return nil
	}
}

func convertPortToInt(port string) (int, error) {
	ports := strings.Split(port, ":")
	if len(ports) != 2 {
		return 0, fmt.Errorf("port not found in string (%v)", ports)
	}
	portInt, err := strconv.Atoi(ports[1])
	if err != nil {
		return 0, err
	}
	return portInt, nil
}
