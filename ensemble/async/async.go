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
	"strings"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"microcks.io/testcontainers-go/ensemble/async/connection/generic"
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
type Option func(*MicrocksAsyncMinionContainer) error

// ContainerOptions represents the container options
type ContainerOptions struct {
	list []testcontainers.ContainerCustomizer
}

// Add adds an option to the list
func (co *ContainerOptions) Add(opt testcontainers.ContainerCustomizer) {
	co.list = append(co.list, opt)
}

// MicrocksAsyncMinionContainer represents the Microcks Async Minion container type used in the module.
type MicrocksAsyncMinionContainer struct {
	testcontainers.Container

	containerOptions ContainerOptions
}

// Deprecated: use Run instead
// RunContainer creates an instance of the MicrocksAsyncMinionContainer type.
func RunContainer(ctx context.Context, microcksHostPort string, opts ...testcontainers.ContainerCustomizer) (*MicrocksAsyncMinionContainer, error) {
	return Run(ctx, DefaultImage, microcksHostPort, opts...)
}

// Run creates an instance of the MicrocksAsyncMinionContainer type.
func Run(ctx context.Context, image string, microcksHostPort string, opts ...testcontainers.ContainerCustomizer) (*MicrocksAsyncMinionContainer, error) {
	req := testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        image,
			ExposedPorts: []string{DefaultHttpPort},
			WaitingFor:   wait.ForLog("Profile prod activated"),
			Env: map[string]string{
				"MICROCKS_HOST_PORT": microcksHostPort,
				"ASYNC_PROTOCOLS":    "",
			},
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

	return &MicrocksAsyncMinionContainer{Container: container}, nil
}

// WithNetwork allows to add a custom network.
// Deprecated: Use network.WithNetwork from testcontainers instead.
func WithNetwork(networkName string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Networks = append(req.Networks, networkName)

		return nil
	}
}

// WithNetworkAlias allows to add a custom network alias for a specific network.
// Deprecated: Use network.WithNetwork from testcontainers instead.
func WithNetworkAlias(networkName, networkAlias string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.NetworkAliases == nil {
			req.NetworkAliases = make(map[string][]string)
		}
		req.NetworkAliases[networkName] = []string{networkAlias}

		return nil
	}
}

// WithEnv allows to add an environment variable.
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
func WithKafkaConnection(connection kafka.Connection) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.Env == nil {
			req.Env = make(map[string]string)
		}
		req.Env["KAFKA_BOOTSTRAP_SERVER"] = connection.BootstrapServers
		addProtocol(req, "KAFKA")

		return nil
	}
}

// WithKafkaConnection connects the MicrocksAsyncMinionContainer to a MQTT broker to allow MQTT messages mocking.
func WithMQTTConnection(connection generic.Connection) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.Env == nil {
			req.Env = make(map[string]string)
		}
		req.Env["MQTT_SERVER"] = connection.Server
		req.Env["MQTT_USERNAME"] = connection.Username
		req.Env["MQTT_PASSWORD"] = connection.Password
		addProtocol(req, "MQTT")

		return nil
	}
}

// WSMockEndpoint gets the exposed mock endpoints for a WebSocket Service.
func (container *MicrocksAsyncMinionContainer) WSMockEndpoint(ctx context.Context, service, version, operationName string) (string, error) {
	// Get the container host.
	host, err := container.Host(ctx)
	if err != nil {
		return "", err
	}

	// Get the container mapped port.
	natPort, err := container.MappedPort(ctx, DefaultHttpPort)
	if err != nil {
		return "", err
	}
	port := natPort.Port()

	// Format service.
	service = strings.ReplaceAll(service, " ", "+")

	// Format version.
	version = strings.ReplaceAll(version, " ", "+")

	// Format operationName.
	if strings.Index(operationName, " ") != -1 {
		operationName = strings.Split(operationName, " ")[1]
	}

	return fmt.Sprintf(
		"ws://%s:%s/api/ws/%s/%s/%s",
		host,
		port,
		service,
		version,
		operationName,
	), nil
}

// KafkaMockTopic gets the exposed mock topic for a Kafka Service.
func (container *MicrocksAsyncMinionContainer) KafkaMockTopic(service, version, operationName string) string {
	// Format operationName.
	if strings.Index(operationName, " ") != -1 {
		operationName = strings.Split(operationName, " ")[1]
	}
	operationName = strings.ReplaceAll(operationName, "/", "-")

	// Format service.
	r := strings.NewReplacer(" ", "", "-", "")
	service = r.Replace(service)

	return fmt.Sprintf("%s-%s-%s", service, version, operationName)
}

func addProtocol(req *testcontainers.GenericContainerRequest, protocol string) {
	if _, ok := req.Env["ASYNC_PROTOCOLS"]; !ok {
		req.Env["ASYNC_PROTOCOLS"] = ""
	}
	if strings.Index(req.Env["ASYNC_PROTOCOLS"], ","+protocol) == -1 {
		req.Env["ASYNC_PROTOCOLS"] += "," + protocol
	}
}
