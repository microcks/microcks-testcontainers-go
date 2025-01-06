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
package ensemble

import (
	"context"
	"strings"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"microcks.io/go-client"
	microcks "microcks.io/testcontainers-go"
	"microcks.io/testcontainers-go/ensemble/async"
	"microcks.io/testcontainers-go/ensemble/async/connection/generic"
	"microcks.io/testcontainers-go/ensemble/async/connection/kafka"
	"microcks.io/testcontainers-go/ensemble/postman"
)

// Option represents an option to pass to the ensemble.
type Option func(*MicrocksContainersEnsemble) error

// ContainerOptions represents the container options.
type ContainerOptions struct {
	list []testcontainers.ContainerCustomizer
}

// Add adds an option to the list.
func (co *ContainerOptions) Add(opt testcontainers.ContainerCustomizer) {
	co.list = append(co.list, opt)
}

// MicrocksContainersEnsemble represents the ensemble of containers.
type MicrocksContainersEnsemble struct {
	ctx context.Context

	network *testcontainers.DockerNetwork

	hostAccessPorts []int

	microcksContainer        *microcks.MicrocksContainer
	microcksContainerImage   string
	microcksContainerOptions ContainerOptions

	postmanEnabled          bool
	postmanContainer        *postman.PostmanContainer
	postmanContainerImage   string
	postmanContainerOptions ContainerOptions

	asyncEnabled                bool
	asyncMinionContainer        *async.MicrocksAsyncMinionContainer
	asyncMinionContainerImage   string
	asyncMinionContainerOptions ContainerOptions
}

// GetNetwork returns the ensemble network.
func (ec *MicrocksContainersEnsemble) GetNetwork() *testcontainers.DockerNetwork {
	return ec.network
}

// GetMicrocksContainer returns the Microcks container.
func (ec *MicrocksContainersEnsemble) GetMicrocksContainer() *microcks.MicrocksContainer {
	return ec.microcksContainer
}

// GetPostmanContainer returns the Postman container.
func (ec *MicrocksContainersEnsemble) GetPostmanContainer() *postman.PostmanContainer {
	return ec.postmanContainer
}

// GetAsyncMinionContainer returns the Async Minion container.
func (ec *MicrocksContainersEnsemble) GetAsyncMinionContainer() *async.MicrocksAsyncMinionContainer {
	return ec.asyncMinionContainer
}

// Terminate helps to terminate all containers.
func (ec *MicrocksContainersEnsemble) Terminate(ctx context.Context) error {
	// Main Microcks container.
	if err := ec.microcksContainer.Terminate(ctx); err != nil {
		return err
	}

	// Postman container.
	if ec.postmanEnabled {
		if err := ec.postmanContainer.Terminate(ctx); err != nil {
			return err
		}
	}

	// Async Microcks minion container.
	if ec.asyncEnabled {
		if err := ec.asyncMinionContainer.Terminate(ctx); err != nil {
			return err
		}
	}

	return nil
}

// RunContainers creates instances of the Microcks Ensemble.
// Using sequential start to avoid resource contention on CI systems with weaker hardware.
func RunContainers(ctx context.Context, opts ...Option) (*MicrocksContainersEnsemble, error) {
	var err error

	ensemble := &MicrocksContainersEnsemble{ctx: ctx}

	// Options.
	defaults := []Option{WithDefaultNetwork()}
	options := append(defaults, opts...)
	for _, opt := range options {
		if err = opt(ensemble); err != nil {
			return nil, err
		}
	}

	// Set microcks container env variables.
	testCallbackURL := strings.Join([]string{"http://", microcks.DefaultNetworkAlias, ":8080"}, "")
	postmanRunnerURL := strings.Join([]string{"http://", postman.DefaultNetworkAlias, ":3000"}, "")
	asyncMinionURL := strings.Join([]string{"http://", async.DefaultNetworkAlias, ":8081"}, "")

	ensemble.microcksContainerOptions.Add(microcks.WithEnv("TEST_CALLBACK_URL", testCallbackURL))
	ensemble.microcksContainerOptions.Add(microcks.WithEnv("POSTMAN_RUNNER_URL", postmanRunnerURL))
	ensemble.microcksContainerOptions.Add(microcks.WithEnv("ASYNC_MINION_URL", asyncMinionURL))

	// Start default Microcks container.
	if len(ensemble.hostAccessPorts) > 0 {
		ensemble.microcksContainerOptions.Add(
			microcks.WithHostAccessPorts(ensemble.hostAccessPorts),
		)
	}
	if ensemble.microcksContainerImage == "" {
		ensemble.microcksContainerImage = microcks.DefaultImage
	}
	ensemble.microcksContainer, err = microcks.Run(ctx, ensemble.microcksContainerImage, ensemble.microcksContainerOptions.list...)
	if err != nil {
		return nil, err
	}

	// Start Postman container if enabled.
	if ensemble.postmanEnabled {
		ensemble.postmanContainer, err = postman.Run(ctx, ensemble.postmanContainerImage, ensemble.postmanContainerOptions.list...)
		if err != nil {
			return nil, err
		}
	}

	// Start Microcks async minion container if enabled.
	if ensemble.asyncEnabled {
		microcksHostPort := strings.Join([]string{microcks.DefaultNetworkAlias, ":8080"}, "")
		ensemble.asyncMinionContainer, err = async.Run(ctx, ensemble.asyncMinionContainerImage, microcksHostPort, ensemble.asyncMinionContainerOptions.list...)
		if err != nil {
			return nil, err
		}
	}

	return ensemble, nil
}

// WithMicrocksImage helps to use specific Microcks image.
func WithMicrocksImage(image string) Option {
	return func(e *MicrocksContainersEnsemble) error {
		//e.microcksContainerOptions.Add(testcontainers.WithImage(image))
		e.microcksContainerImage = image
		return nil
	}
}

// WithAsynncFature enables the Async Feature container with default container image (deduced from Microcks main one).
func WithAsyncFeature() Option {
	return func(e *MicrocksContainersEnsemble) error {
		e.asyncMinionContainerImage = async.DefaultImage
		e.asyncEnabled = true
		return nil
	}
}

// WithAsyncFeatureImage enabled the Async Feature container with specific image.
func WithAsyncFeatureImage(image string) Option {
	return func(e *MicrocksContainersEnsemble) error {
		//e.asyncMinionContainerOptions.Add(testcontainers.WithImage(image))
		e.asyncMinionContainerImage = image
		e.asyncEnabled = true
		return nil
	}
}

// WithPostman allows to enable Postman container.
func WithPostman() Option {
	return func(e *MicrocksContainersEnsemble) error {
		e.postmanContainerImage = postman.DefaultImage
		e.postmanEnabled = true
		return nil
	}
}

// WithPostmanImage helps to use specific Postman image.
func WithPostmanImage(image string) Option {
	return func(e *MicrocksContainersEnsemble) error {
		//e.postmanContainerOptions.Add(testcontainers.WithImage(image))
		e.postmanContainerImage = image
		e.postmanEnabled = true
		return nil
	}
}

// WithDefaultNetwork allows to use a default network.
func WithDefaultNetwork() Option {
	return func(e *MicrocksContainersEnsemble) (err error) {
		e.network, err = network.New(e.ctx, network.WithCheckDuplicate())
		if err != nil {
			return err
		}

		e.microcksContainerOptions.Add(microcks.WithNetwork(e.network.Name))
		e.microcksContainerOptions.Add(microcks.WithNetworkAlias(e.network.Name, microcks.DefaultNetworkAlias))
		e.postmanContainerOptions.Add(postman.WithNetwork(e.network.Name))
		e.postmanContainerOptions.Add(postman.WithNetworkAlias(e.network.Name, postman.DefaultNetworkAlias))
		e.asyncMinionContainerOptions.Add(async.WithNetwork(e.network.Name))
		e.asyncMinionContainerOptions.Add(async.WithNetworkAlias(e.network.Name, async.DefaultNetworkAlias))

		return nil
	}
}

// WithNetwork allows to define the network.
func WithNetwork(network *testcontainers.DockerNetwork) Option {
	return func(e *MicrocksContainersEnsemble) error {
		e.network = network
		e.microcksContainerOptions.Add(microcks.WithNetwork(e.network.Name))
		e.microcksContainerOptions.Add(microcks.WithNetworkAlias(e.network.Name, microcks.DefaultNetworkAlias))
		e.postmanContainerOptions.Add(postman.WithNetwork(e.network.Name))
		e.postmanContainerOptions.Add(postman.WithNetworkAlias(e.network.Name, postman.DefaultNetworkAlias))
		e.asyncMinionContainerOptions.Add(async.WithNetwork(e.network.Name))
		e.asyncMinionContainerOptions.Add(async.WithNetworkAlias(e.network.Name, async.DefaultNetworkAlias))
		return nil
	}
}

// WithMainArtifact provides paths to artifacts that will be imported as main or main
// ones within the Microcks container.
// Once it will be started and healthy.
func WithMainArtifact(artifactFilePath string) Option {
	return func(e *MicrocksContainersEnsemble) error {
		e.microcksContainerOptions.Add(microcks.WithMainArtifact(artifactFilePath))
		return nil
	}
}

// WithSecondaryArtifact provides paths to artifacts that will be imported as main or main
// ones within the Microcks container.
// Once it will be started and healthy.
func WithSecondaryArtifact(artifactFilePath string) Option {
	return func(e *MicrocksContainersEnsemble) error {
		e.microcksContainerOptions.Add(microcks.WithSecondaryArtifact(artifactFilePath))
		return nil
	}
}

// WithHostAccessPorts helps to open connections between Microcks, Postman or Microcks async
// to the user's host ports.
func WithHostAccessPorts(hostAccessPorts []int) Option {
	return func(e *MicrocksContainersEnsemble) error {
		e.hostAccessPorts = hostAccessPorts
		return nil
	}
}

// WithKafkaConnection configures the Kafka connection.
func WithKafkaConnection(connection kafka.Connection) Option {
	return func(e *MicrocksContainersEnsemble) error {
		e.asyncMinionContainerOptions.Add(async.WithKafkaConnection(connection))
		return nil
	}
}

// WithMQTTConnection configures a connection to a MQTT Broker.
func WithMQTTConnection(connection generic.Connection) Option {
	return func(e *MicrocksContainersEnsemble) error {
		e.asyncMinionContainerOptions.Add(async.WithMQTTConnection(connection))
		return nil
	}
}

// WithAMQPConnection configures a connection to an AMQP/RabbitMQ Broker.
func WithAMQPConnection(connection generic.Connection) Option {
	return func(e *MicrocksContainersEnsemble) error {
		e.asyncMinionContainerOptions.Add(async.WithAMQPConnection(connection))
		return nil
	}
}

// WithSecret creates a new secret.
func WithSecret(s client.Secret) Option {
	return func(e *MicrocksContainersEnsemble) error {
		e.asyncMinionContainerOptions.Add(microcks.WithSecret(s))
		return nil
	}
}
