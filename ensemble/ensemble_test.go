package ensemble_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"microcks.io/testcontainers-go/ensemble"
	"microcks.io/testcontainers-go/internal/test"
)

func TestMockingFunctionalityAtStartup(t *testing.T) {
	ctx := context.Background()

	ec, err := ensemble.RunContainers(ctx,
		ensemble.WithMainArtifact("../testdata/apipastries-openapi.yaml"),
		ensemble.WithSecondaryArtifact("../testdata/apipastries-postman-collection.json"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
	test.MockEndpoints(t, ctx, ec.GetMicrocksContainer())

	test.MicrocksMockingFunctionality(t, ctx, ec.GetMicrocksContainer())
}

func TestPostmanContractTestingFunctionality(t *testing.T) {
	ctx := context.Background()

	ec, err := ensemble.RunContainers(ctx,
		ensemble.WithMainArtifact("../testdata/apipastries-openapi.yaml"),
		ensemble.WithSecondaryArtifact("../testdata/apipastries-postman-collection.json"),
		ensemble.WithPostman(true),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
	test.MockEndpoints(t, ctx, ec.GetMicrocksContainer())

	test.MicrocksMockingFunctionality(t, ctx, ec.GetMicrocksContainer())
}
