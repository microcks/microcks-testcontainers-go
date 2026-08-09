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
package microcks_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	microcks "microcks.io/testcontainers-go"
	"microcks.io/testcontainers-go/internal/test"
)

func TestMockingFunctionalityAtStartup(t *testing.T) {
	ctx := context.Background()

	microcksContainer, err := microcks.RunContainer(ctx,
		testcontainers.WithImage("quay.io/microcks/microcks-uber:nightly"),
		microcks.WithDebugLogLevel(),
		microcks.WithMainArtifact("testdata/apipastries-openapi.yaml"),
		microcks.WithSecondaryArtifact("testdata/apipastries-postman-collection.json"),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := microcksContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Checking DEBUG 1 in logs.
	readCloser, err := microcksContainer.Logs(ctx)
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(readCloser)
	require.NoError(t, err)
	readCloser.Close()

	require.Contains(t, buf.String(), "DEBUG 1", "Expected to find 'DEBUG 1' log line in Microcks logs")

	// Checking mocking functionality.
	test.ConfigRetrieval(t, ctx, microcksContainer)
	test.MockEndpoints(t, ctx, microcksContainer)

	test.MicrocksMockingFunctionality(t, ctx, microcksContainer)
}

func TestMockingFunctionality(t *testing.T) {
	ctx := context.Background()

	microcksContainer, err := microcks.Run(ctx, "quay.io/microcks/microcks-uber:nightly")
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := microcksContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Loading artifacts.
	status, err := microcksContainer.ImportAsMainArtifact(ctx, filepath.Join("testdata", "apipastries-openapi.yaml"))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	status, err = microcksContainer.ImportAsSecondaryArtifact(ctx, filepath.Join("testdata", "apipastries-postman-collection.json"))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	test.ConfigRetrieval(t, ctx, microcksContainer)
	test.MockEndpoints(t, ctx, microcksContainer)

	test.MicrocksMockingFunctionality(t, ctx, microcksContainer)
}

func TestContractTestingFunctionality(t *testing.T) {
	ctx := context.Background()

	var networkName = "microcks-network"
	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name: networkName,
		},
	})
	require.NoError(t, err, "cannot create network")

	defer func() {
		_ = network.Remove(ctx)
	}()

	microcksContainer, err := microcks.RunContainer(ctx,
		testcontainers.WithImage("quay.io/microcks/microcks-uber:nightly"),
		microcks.WithNetwork(networkName),
	)
	require.NoError(t, err)

	badImpl, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:    "quay.io/microcks/contract-testing-demo:01",
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"bad-impl"},
			},
			WaitingFor: wait.ForLog("Example app listening on port 3001"),
		},
		Started: true,
	})
	require.NoError(t, err)

	goodImpl, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:    "quay.io/microcks/contract-testing-demo:02",
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"good-impl"},
			},
			WaitingFor: wait.ForLog("Example app listening on port 3002"),
		},
		Started: true,
	})
	require.NoError(t, err)

	t.Cleanup(func() {
		if err := microcksContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
		if err := badImpl.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
		if err := goodImpl.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Loading artifacts.
	status, err := microcksContainer.ImportAsMainArtifact(ctx, filepath.Join("testdata", "apipastries-openapi.yaml"))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	status, err = microcksContainer.ImportAsSecondaryArtifact(ctx, filepath.Join("testdata", "apipastries-postman-collection.json"))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	test.ConfigRetrieval(t, ctx, microcksContainer)

	test.AssertBadImplementation(t, ctx, microcksContainer)
	test.AssertGoodImplementation(t, ctx, microcksContainer)

	test.PrintMicrocksContainerLogs(t, ctx, microcksContainer)
}

func TestWebhookFunctionality(t *testing.T) {
	ctx := context.Background()

	var mu sync.Mutex
	var received []string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		mu.Lock()
		received = append(received, string(body))
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Extract the port from the httptest server so we can expose it to the container.
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)
	port, err := strconv.Atoi(serverURL.Port())
	require.NoError(t, err)

	microcksContainer, err := microcks.Run(ctx, "quay.io/microcks/microcks-uber:nightly",
		microcks.WithHostAccessPorts([]int{port}),
		microcks.WithMainArtifact("testdata/petstore-webhooks-openapi.yaml"),
		microcks.WithWebhookRegistration(microcks.WebhookCoordinates{
			ServiceId:     "Petstore Webhooks:2.0.0",
			OperationName: "newPet WEBHOOK",
			TargetUrl:     fmt.Sprintf("http://host.testcontainers.internal:%d", port),
		}),
	)
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := microcksContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Poll for webhook messages (Microcks pushes them every 3s).
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		if len(received) > 0 {
			mu.Unlock()
			break
		}
		mu.Unlock()
		time.Sleep(100 * time.Millisecond)
	}

	mu.Lock()
	defer mu.Unlock()

	require.NotEmpty(t, received, "webhook server should have received at least one message")

	// Check the content of every received message matches the 'Rusty' example.
	for _, msg := range received {
		var payload struct {
			Id   int    `json:"id"`
			Name string `json:"name"`
			Tag  string `json:"tag"`
		}
		err := json.Unmarshal([]byte(msg), &payload)
		require.NoError(t, err, "received message should be valid JSON")

		require.Equal(t, "Rusty", payload.Name)
		require.True(t, payload.Id >= 1 && payload.Id <= 10,
			"id should be a random int between 1 and 10 but was %d", payload.Id)
		require.Contains(t, []string{"cat", "dog"}, payload.Tag,
			"tag should be either 'cat' or 'dog' but was %q", payload.Tag)
	}
}
