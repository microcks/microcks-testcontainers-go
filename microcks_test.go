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
package microcks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	client "github.com/microcks/microcks-go-client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
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

	// Loading artifacts
	status, err := microcksContainer.ImportAsMainArtifact("test-resources/apipastries-openapi.yaml")
	if err != nil {
		log.Fatal(err)
	}
	require.Equal(t, http.StatusCreated, status)

	status, err = microcksContainer.ImportAsSecondaryArtifact("test-resources/apipastries-postman-collection.json")
	if err != nil {
		log.Fatal(err)
	}
	require.Equal(t, http.StatusCreated, status)

	testConfigRetrieval(t, ctx, microcksContainer)
	testMockEndpoints(t, ctx, microcksContainer)

	testMicrocksMockingFunctionality(t, ctx, microcksContainer)

	//printMicrocksContainerLogs(ctx, microcksContainer);
}

func TestContractTestingFunctionnality(t *testing.T) {
	ctx := context.Background()

	var networkName = "microcks-network"
	network, err := testcontainers.GenericNetwork(ctx, testcontainers.GenericNetworkRequest{
		NetworkRequest: testcontainers.NetworkRequest{
			Name: networkName,
		},
	})
	if err != nil {
		t.Fatal("Cannot create network")
	}
	defer func() {
		_ = network.Remove(ctx)
	}()

	// createMicrocksContainer {
	microcksContainer, err := RunContainer(ctx, customizeMicrocksContainer("quay.io/microcks/microcks-uber:nightly", networkName))
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
	// }

	// Loading artifacts
	status, err := microcksContainer.ImportAsMainArtifact("test-resources/apipastries-openapi.yaml")
	if err != nil {
		log.Fatal(err)
	}
	require.Equal(t, http.StatusCreated, status)

	status, err = microcksContainer.ImportAsSecondaryArtifact("test-resources/apipastries-postman-collection.json")
	if err != nil {
		log.Fatal(err)
	}
	require.Equal(t, http.StatusCreated, status)

	testConfigRetrieval(t, ctx, microcksContainer)

	testMicrocksContractTestingFunctionality(t, ctx, microcksContainer)

	printMicrocksContainerLogs(ctx, microcksContainer)
}

func testConfigRetrieval(t *testing.T, ctx context.Context, microcksContainer *MicrocksContainer) {
	// HttpEndpoint {
	uri := microcksContainer.HttpEndpoint(ctx)
	resp, err := http.Get(uri + "/api/keycloak/config")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	// }
}

func testMockEndpoints(t *testing.T, ctx context.Context, microcksContainer *MicrocksContainer) {
	// MockEndpoints {
	baseApiUrl := microcksContainer.SoapMockEndpoint(ctx, "Pastries Service", "1.0")
	require.Equal(t, microcksContainer.HttpEndpoint(ctx)+"/soap/Pastries Service/1.0", baseApiUrl)

	baseApiUrl = microcksContainer.RestMockEndpoint(ctx, "API Pastries", "0.0.1")
	require.Equal(t, microcksContainer.HttpEndpoint(ctx)+"/rest/API Pastries/0.0.1", baseApiUrl)

	baseApiUrl = microcksContainer.GrapQLMockEndpoint(ctx, "Pastries Graph", "1")
	require.Equal(t, microcksContainer.HttpEndpoint(ctx)+"/graphql/Pastries Graph/1", baseApiUrl)

	baseGrpcUrl := microcksContainer.GrpcMockEndpoint(ctx)
	ip, _ := microcksContainer.Host(ctx)
	port, _ := microcksContainer.MappedPort(ctx, defaultGrpcPort)
	require.Equal(t, "grpc://"+ip+":"+port.Port(), baseGrpcUrl)
	// }
}

func testMicrocksMockingFunctionality(t *testing.T, ctx context.Context, microcksContainer *MicrocksContainer) {
	baseApiUrl := microcksContainer.RestMockEndpoint(ctx, "API Pastries", "0.0.1")

	resp, err := http.Get(baseApiUrl + "/pastries/Millefeuille")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Unmarshal body using a generic interface
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		panic(err.Error())
	}

	var pastry = map[string]string{}
	json.Unmarshal([]byte(body), &pastry)

	require.Equal(t, "Millefeuille", pastry["name"])

	// Check that mock from secondary artifact has been loaded.
	resp, err = http.Get(baseApiUrl + "/pastries/Eclair Chocolat")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Unmarshal body using a generic interface
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		panic(err.Error())
	}

	pastry = map[string]string{}
	json.Unmarshal([]byte(body), &pastry)

	require.Equal(t, "Eclair Chocolat", pastry["name"])
}

func testMicrocksContractTestingFunctionality(t *testing.T, ctx context.Context, microcksContainer *MicrocksContainer) {
	// Build a new TestRequest.
	testRequest := client.TestRequest{
		ServiceId:    "API Pastries:0.0.1",
		RunnerType:   client.TestRunnerTypeOPENAPISCHEMA,
		TestEndpoint: "http://bad-impl:3001",
		Timeout:      2000,
	}

	testResult, err := microcksContainer.TestEndpoint(&testRequest)
	require.NoError(t, err)

	println(testResult.Success)

	require.False(t, testResult.Success)
	require.Equal(t, "http://bad-impl:3001", testResult.TestedEndpoint)

	require.Equal(t, 3, len(*testResult.TestCaseResults))
	for _, r := range *testResult.TestCaseResults {
		require.False(t, r.Success)
	}

	// Switch endpoint to the correct implementation.
	testRequest = client.TestRequest{
		ServiceId:    "API Pastries:0.0.1",
		RunnerType:   client.TestRunnerTypeOPENAPISCHEMA,
		TestEndpoint: "http://good-impl:3002",
		Timeout:      2000,
	}

	testResult, err = microcksContainer.TestEndpoint(&testRequest)
	require.NoError(t, err)

	println(testResult.Success)

	require.True(t, testResult.Success)
	require.Equal(t, "http://good-impl:3002", testResult.TestedEndpoint)

	require.Equal(t, 3, len(*testResult.TestCaseResults))
	for _, r := range *testResult.TestCaseResults {
		require.True(t, r.Success)
	}
}

func customizeMicrocksContainer(image string, network string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Image = image
		req.Networks = []string{network}
	}
}

func printMicrocksContainerLogs(ctx context.Context, microcksContainer *MicrocksContainer) {
	readCloser, err := microcksContainer.Logs(ctx)
	// example to read data
	buf := new(bytes.Buffer)
	numOfByte, err := buf.ReadFrom(readCloser)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	readCloser.Close()
	fmt.Printf("Read: %d bytes, content is: %q", numOfByte, buf.String())
}
