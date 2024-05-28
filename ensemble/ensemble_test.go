package ensemble_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"microcks.io/testcontainers-go/ensemble"
	"microcks.io/testcontainers-go/internal/test"
)

func TestMockingFunctionalityAtStartup(t *testing.T) {
	ctx := context.Background()

	// Ensemble.
	ec, err := ensemble.RunContainers(ctx,
		ensemble.WithMainArtifact("../testdata/apipastries-openapi.yaml"),
		ensemble.WithSecondaryArtifact("../testdata/apipastries-postman-collection.json"),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
	test.MockEndpoints(t, ctx, ec.GetMicrocksContainer())
	test.MicrocksMockingFunctionality(t, ctx, ec.GetMicrocksContainer())
}

func TestPostmanContractTestingFunctionality(t *testing.T) {
	ctx := context.Background()

	// Ensemble.
	ec, err := ensemble.RunContainers(
		ctx,
		ensemble.WithMainArtifact("../testdata/apipastries-openapi.yaml"),
		ensemble.WithSecondaryArtifact("../testdata/apipastries-postman-collection.json"),
		ensemble.WithPostman(true),
	)
	require.NoError(t, err)
	networkName := ec.GetNetwork().Name

	// Demo pastry bad implementation.
	badImpl, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:    "quay.io/microcks/contract-testing-demo:02",
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"bad-impl"},
			},
			WaitingFor: wait.ForLog("Example app listening on port 3002"),
		},
		Started: true,
	})
	require.NoError(t, err)

	// Demo pastry good implementation.
	goodImpl, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:    "quay.io/microcks/contract-testing-demo:03",
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"good-impl"},
			},
			WaitingFor: wait.ForLog("Example app listening on port 3003"),
		},
		Started: true,
	})
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.GetMicrocksContainer().Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
		if err := badImpl.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
		if err := goodImpl.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
	test.MicrocksContractTestingFunctionality(
		t,
		ctx,
		ec.GetMicrocksContainer(),
		badImpl,
		goodImpl,
	)
}

func TestAsyncFeatureSetup(t *testing.T) {
	ctx := context.Background()

	// Ensemble.
	ec, err := ensemble.RunContainers(
		ctx,
		ensemble.WithAsyncFeature(),
		ensemble.WithHostAccessPorts([]int{8080}),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
}

func TestAsyncFeatureMockingFunctionality(t *testing.T) {
	ctx := context.Background()

	// Ensemble.
	ec, err := ensemble.RunContainers(
		ctx,
		ensemble.WithAsyncFeature(),
		ensemble.WithMainArtifact("../testdata/pastry-orders-asyncapi.yaml"),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
	test.MicrocksAsyncMockingFunctionality(t, ctx, ec.GetAsyncMinionContainer())
}
