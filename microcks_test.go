package microcks

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go"
)

func TestMockingFunctionality(t *testing.T) {
	ctx := context.Background()

	// createMicrocksContainer {
	microcksContainer, err := RunContainer(ctx, testcontainers.WithImage("quay.io/microcks/microcks-uber:nightly"))
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := microcksContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})
	// }

	assertConfigRetrieval(t, ctx, microcksContainer)
}

func assertConfigRetrieval(t *testing.T, ctx context.Context, microcksContainer *MicrocksContainer) {
	// HttpEndpoint {
	uri := microcksContainer.HttpEndpoint(ctx)
	resp, err := http.Get(uri + "/api/keycloak/config")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	// }
}
