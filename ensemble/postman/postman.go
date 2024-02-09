package postman

import (
	"context"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const (
	defaultImage = "quay.io/microcks/microcks-postman-runtime:latest"

	// DefaultHTTPPort represents the default Postman HTTP port
	DefaultHTTPPort = "6000/tcp"
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
			WaitingFor:   wait.ForLog("Started Postman"),
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
