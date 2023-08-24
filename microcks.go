package microcks

import (
	"context"
	"fmt"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const defaultImage = "quay.io/microcks/microcks-uber:latest"

const defaultHttpPort = "8080/tcp"
const defaultGrpcPort = "9090/tcp"

type MicrocksContainer struct {
	testcontainers.Container
}

func (container *MicrocksContainer) HttpEndpoint(ctx context.Context) string {
	ip, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, defaultHttpPort)
	return fmt.Sprintf("http://%s:%s", ip, port.Port())
}

func (container *MicrocksContainer) ImportAsMainArtifact() {

}

func (container *MicrocksContainer) ImportAsSecondaryArtifact() {

}

func (container *MicrocksContainer) TestEndpoint() {

}

// RunContainer creates an instance of the MicrocksContainer type.
func RunContainer(ctx context.Context, opts ...testcontainers.ContainerCustomizer) (*MicrocksContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        defaultImage,
		ExposedPorts: []string{defaultHttpPort, defaultGrpcPort},
		WaitingFor:   wait.ForLog("Started MicrocksApplication"),
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

	return &MicrocksContainer{Container: container}, nil
}
