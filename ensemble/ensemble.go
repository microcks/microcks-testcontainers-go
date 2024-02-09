package ensemble

import (
	"context"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	microcks "microcks.io/testcontainers-go"
	"microcks.io/testcontainers-go/ensemble/postman"
)

const (
	defaultNetworkAlias = "microcks"
)

// Option represents an option to pass to the ensemble
type Option func(*MicrocksEnsembleContainers) error

// ContainerOptions represents the container options
type ContainerOptions struct {
	list []testcontainers.ContainerCustomizer
}

// Add adds an option to the list
func (co *ContainerOptions) Add(opt testcontainers.ContainerCustomizer) {
	co.list = append(co.list, opt)
}

// MicrocksEnsembleContainers represents the ensemble of containers
type MicrocksEnsembleContainers struct {
	ctx context.Context

	network                    *testcontainers.DockerNetwork
	primaryArtifactFilePath    string
	secondaryArtifactsFilePath string

	microcksContainer        *microcks.MicrocksContainer
	microcksContainerOptions ContainerOptions

	postmanContainer        *postman.PostmanContainer
	postmanContainerOptions ContainerOptions
}

// GetMicrocksContainers returns the Microcks container
func (ec *MicrocksEnsembleContainers) GetMicrocksContainers() *microcks.MicrocksContainer {
	return ec.microcksContainer
}

// GetPostmanContainer returns the Postman container
func (ec *MicrocksEnsembleContainers) GetPostmanContainer() *postman.PostmanContainer {
	return ec.postmanContainer
}

// RunEnsembleContainers creates instances of the Microcks and necessaries tools.
// Using sequential start to avoid resource contention on CI systems with weaker hardware.
func RunEnsembleContainers(ctx context.Context, opts ...Option) (*MicrocksEnsembleContainers, error) {
	var err error

	ensemble := &MicrocksEnsembleContainers{
		ctx: ctx,
	}

	// Options
	defaults := []Option{WithDefaultNetwork()}
	options := append(defaults, opts...)
	for _, opt := range options {
		if err = opt(ensemble); err != nil {
			return nil, err
		}
	}

	// Microcks container
	ensemble.microcksContainerOptions.Add(microcks.WithEnv("POSTMAN_RUNNER_URL", "http://postman:3000"))
	ensemble.microcksContainerOptions.Add(microcks.WithEnv("TEST_CALLBACK_URL", "http://microcks:8080"))
	ensemble.microcksContainer, err = microcks.RunContainer(ctx, ensemble.microcksContainerOptions.list...)
	if err != nil {
		return nil, err
	}

	// Postman container
	ensemble.postmanContainer, err = postman.RunContainer(ctx, ensemble.postmanContainerOptions.list...)
	if err != nil {
		return nil, err
	}

	return ensemble, nil
}

// WithDefaultNetwork allows to use a default network
func WithDefaultNetwork() Option {
	return func(e *MicrocksEnsembleContainers) (err error) {
		e.network, err = network.New(e.ctx, network.WithCheckDuplicate())
		if err != nil {
			return err
		}

		e.microcksContainerOptions.Add(microcks.WithNetwork(e.network.Name))
		e.microcksContainerOptions.Add(microcks.WithNetworkAlias(e.network.Name, defaultNetworkAlias))
		e.postmanContainerOptions.Add(postman.WithNetwork(e.network.Name))
		e.postmanContainerOptions.Add(postman.WithNetworkAlias(e.network.Name, defaultNetworkAlias))

		return nil
	}
}

// WithNetwork allows to define the network
func WithNetwork(network *testcontainers.DockerNetwork) Option {
	return func(e *MicrocksEnsembleContainers) error {
		e.network = network
		e.microcksContainerOptions.Add(microcks.WithNetwork(e.network.Name))
		e.microcksContainerOptions.Add(microcks.WithNetworkAlias(e.network.Name, defaultNetworkAlias))
		e.postmanContainerOptions.Add(postman.WithNetwork(e.network.Name))
		e.postmanContainerOptions.Add(postman.WithNetworkAlias(e.network.Name, defaultNetworkAlias))
		return nil
	}
}

// WithMainArtifact provides paths to artifacts that will be imported as main or main
// ones within the Microcks container.
// Once it will be started and healthy.
func WithMainArtifact(artifactFilePath string) Option {
	return func(e *MicrocksEnsembleContainers) error {
		e.microcksContainerOptions.Add(microcks.WithMainArtifact(artifactFilePath))
		return nil
	}
}

// WithSecondaryArtifact provides paths to artifacts that will be imported as main or main
// ones within the Microcks container.
// Once it will be started and healthy.
func WithSecondaryArtifact(artifactFilePath string) Option {
	return func(e *MicrocksEnsembleContainers) error {
		e.microcksContainerOptions.Add(microcks.WithSecondaryArtifact(artifactFilePath))
		return nil
	}
}
