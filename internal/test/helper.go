package test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"microcks.io/go-client"
	microcks "microcks.io/testcontainers-go"
)

// ConfigRetrieval tests the configuration
func ConfigRetrieval(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	uri, err := microcksContainer.HttpEndpoint(ctx)
	require.NoError(t, err)

	resp, err := http.Get(uri + "/api/keycloak/config")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// MockEndpoints tests the mock endpoints
func MockEndpoints(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
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

// MicrocksMockingFunctionality tests the Microcks mocking functionality
func MicrocksMockingFunctionality(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
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

// AssertBadImplementation helps to assert the endpoint with a bad implementation
func AssertBadImplementation(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
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

// AssertGoodImplementation helps to assert the endpoint with a good implementation
func AssertGoodImplementation(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
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

// MicrocksContractTestingFunctionality helps to assert contract testing functionality
func MicrocksContractTestingFunctionality(
	t *testing.T,
	ctx context.Context,
	mc *microcks.MicrocksContainer,
	badImpl,
	goodImpl testcontainers.Container) {

	// Bad implementation
	testRequestBad := client.TestRequest{
		ServiceId:    "API Pastries:0.0.1",
		RunnerType:   client.TestRunnerTypePOSTMAN,
		TestEndpoint: "http://bad-impl:3002",
		Timeout:      5000,
	}
	testResultBad, errBad := mc.TestEndpoint(ctx, &testRequestBad)
	require.NoError(t, errBad)
	require.False(t, testResultBad.Success)
	require.Equal(t, "http://bad-impl:3002", testResultBad.TestedEndpoint)
	require.Equal(t, 3, len(*testResultBad.TestCaseResults))
	for _, r := range *testResultBad.TestCaseResults {
		require.False(t, r.Success)
	}

	// Good implementation
	testRequestGood := client.TestRequest{
		ServiceId:    "API Pastries:0.0.1",
		RunnerType:   client.TestRunnerTypePOSTMAN,
		TestEndpoint: "http://good-impl:3003",
		Timeout:      5000,
	}
	testResultGood, errGood := mc.TestEndpoint(ctx, &testRequestGood)
	require.NoError(t, errGood)
	require.True(t, testResultGood.Success)
	require.Equal(t, "http://good-impl:3003", testResultGood.TestedEndpoint)
	require.Equal(t, 3, len(*testResultGood.TestCaseResults))
	for _, r := range *testResultGood.TestCaseResults {
		require.True(t, r.Success)
	}
	// TODO: Assert messages
}

// Deprecated: use testcontainers.WithNetwork once it's released.
// WithNetwork is a custom request option that adds a network to a container.
// This is a temporary option until the next release of testcontainers-go, which will include
// this option.
func WithNetwork(network string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) {
		req.Networks = []string{network}
	}
}

// PrintMicrocksContainerLogs prints the Microcks container logs
func PrintMicrocksContainerLogs(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	readCloser, err := microcksContainer.Logs(ctx)
	require.NoError(t, err)

	// example to read data
	buf := new(bytes.Buffer)
	numOfByte, err := buf.ReadFrom(readCloser)
	require.NoError(t, err)

	readCloser.Close()
	t.Logf("Read: %d bytes, content is: %s", numOfByte, buf.String())
}
