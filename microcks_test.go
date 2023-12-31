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
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	client "microcks.io/go-client"
	microcks "microcks.io/testcontainers-go"
)

func TestMockingFunctionality(t *testing.T) {
	ctx := context.Background()

	microcksContainer, err := microcks.RunContainer(ctx, testcontainers.WithImage("quay.io/microcks/microcks-uber:nightly"))
	require.NoError(t, err)
	t.Cleanup(func() {
		if err := microcksContainer.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Loading artifacts
	status, err := microcksContainer.ImportAsMainArtifact(ctx, filepath.Join("testdata", "apipastries-openapi.yaml"))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	status, err = microcksContainer.ImportAsSecondaryArtifact(ctx, filepath.Join("testdata", "apipastries-postman-collection.json"))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	testConfigRetrieval(t, ctx, microcksContainer)
	testMockEndpoints(t, ctx, microcksContainer)

	testMicrocksMockingFunctionality(t, ctx, microcksContainer)
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
		withNetwork(networkName),
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

	// Loading artifacts
	status, err := microcksContainer.ImportAsMainArtifact(ctx, filepath.Join("testdata", "apipastries-openapi.yaml"))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	status, err = microcksContainer.ImportAsSecondaryArtifact(ctx, filepath.Join("testdata", "apipastries-postman-collection.json"))
	require.NoError(t, err)
	require.Equal(t, http.StatusCreated, status)

	testConfigRetrieval(t, ctx, microcksContainer)

	assertBadImplementation(t, ctx, microcksContainer)
	assertGoodImplementation(t, ctx, microcksContainer)

	printMicrocksContainerLogs(t, ctx, microcksContainer)
}

func testConfigRetrieval(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	uri, err := microcksContainer.HttpEndpoint(ctx)
	require.NoError(t, err)

	resp, err := http.Get(uri + "/api/keycloak/config")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

func testMockEndpoints(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	endpoint, err := microcksContainer.HttpEndpoint(ctx)
	require.NoError(t, err)

	baseApiUrl, err := microcksContainer.SoapMockEndpoint(ctx, "Pastries Service", "1.0")
	require.NoError(t, err)
	require.Equal(t, endpoint+"/soap/Pastries Service/1.0", baseApiUrl)

	baseApiUrl, err = microcksContainer.RestMockEndpoint(ctx, "API Pastries", "0.0.1")
	require.NoError(t, err)
	require.Equal(t, endpoint+"/rest/API Pastries/0.0.1", baseApiUrl)

	baseApiUrl, err = microcksContainer.GrapQLMockEndpoint(ctx, "Pastries Graph", "1")
	require.NoError(t, err)
	require.Equal(t, endpoint+"/graphql/Pastries Graph/1", baseApiUrl)

	baseGrpcUrl, err := microcksContainer.GrpcMockEndpoint(ctx)
	require.NoError(t, err)

	ip, err := microcksContainer.Host(ctx)
	require.NoError(t, err)

	port, err := microcksContainer.MappedPort(ctx, microcks.DefaultGrpcPort)
	require.NoError(t, err)
	require.Equal(t, "grpc://"+ip+":"+port.Port(), baseGrpcUrl)
}

func testMicrocksMockingFunctionality(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	baseApiUrl, err := microcksContainer.RestMockEndpoint(ctx, "API Pastries", "0.0.1")
	require.NoError(t, err)

	resp, err := http.Get(baseApiUrl + "/pastries/Millefeuille")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Unmarshal body using a generic interface
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var pastry = map[string]string{}
	json.Unmarshal([]byte(body), &pastry)

	require.Equal(t, "Millefeuille", pastry["name"])

	// Check that mock from secondary artifact has been loaded.
	resp, err = http.Get(baseApiUrl + "/pastries/Eclair Chocolat")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Unmarshal body using a generic interface
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	pastry = map[string]string{}
	json.Unmarshal([]byte(body), &pastry)

	require.Equal(t, "Eclair Chocolat", pastry["name"])
}

// assertions for the endpoint with a bad implementation
func assertBadImplementation(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	// Build a new TestRequest.
	testRequest := client.TestRequest{
		ServiceId:    "API Pastries:0.0.1",
		RunnerType:   client.TestRunnerTypeOPENAPISCHEMA,
		TestEndpoint: "http://bad-impl:3001",
		Timeout:      2000,
	}

	testResult, err := microcksContainer.TestEndpoint(ctx, &testRequest)
	require.NoError(t, err)

	t.Logf("Test Result success is %t", testResult.Success)

	require.False(t, testResult.Success)
	require.Equal(t, "http://bad-impl:3001", testResult.TestedEndpoint)

	require.Equal(t, 3, len(*testResult.TestCaseResults))
	for _, r := range *testResult.TestCaseResults {
		require.False(t, r.Success)
	}

	t0 := (*testResult.TestCaseResults)[0].TestStepResults
	require.True(t, strings.Contains(*(*t0)[0].Message, "object has missing required properties"))
}

// assertions for the endpoint with a good implementation
func assertGoodImplementation(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	// Switch endpoint to the correct implementation.
	testRequest := client.TestRequest{
		ServiceId:    "API Pastries:0.0.1",
		RunnerType:   client.TestRunnerTypeOPENAPISCHEMA,
		TestEndpoint: "http://good-impl:3002",
		Timeout:      2000,
	}

	testResult, err := microcksContainer.TestEndpoint(ctx, &testRequest)
	require.NoError(t, err)

	t.Logf("Test Result success is %t", testResult.Success)

	require.True(t, testResult.Success)
	require.Equal(t, "http://good-impl:3002", testResult.TestedEndpoint)

	require.Equal(t, 3, len(*testResult.TestCaseResults))
	for _, r := range *testResult.TestCaseResults {
		require.True(t, r.Success)
	}
}

// Deprecated: use testcontainers.WithNetwork once it's released.
// withNetwork is a custom request option that adds a network to a container.
// This is a temporary option until the next release of testcontainers-go, which will include
// this option.
func withNetwork(network string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Networks = []string{network}
	}
}

func printMicrocksContainerLogs(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	readCloser, err := microcksContainer.Logs(ctx)
	require.NoError(t, err)

	// example to read data
	buf := new(bytes.Buffer)
	numOfByte, err := buf.ReadFrom(readCloser)
	require.NoError(t, err)

	readCloser.Close()
	t.Logf("Read: %d bytes, content is: %s", numOfByte, buf.String())
}
