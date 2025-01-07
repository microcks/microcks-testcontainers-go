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
package test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"testing"
	"time"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	kafkaTC "github.com/testcontainers/testcontainers-go/modules/kafka"
	"microcks.io/go-client"
	microcks "microcks.io/testcontainers-go"
	"microcks.io/testcontainers-go/ensemble/async"
)

// ConfigRetrieval tests the configuration.
func ConfigRetrieval(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	uri, err := microcksContainer.HttpEndpoint(ctx)
	require.NoError(t, err)

	resp, err := http.Get(uri + "/api/keycloak/config")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)
}

// MockEndpoints tests the mock endpoints.
func MockEndpoints(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	endpoint, err := microcksContainer.HttpEndpoint(ctx)
	require.NoError(t, err)

	baseApiUrl, err := microcksContainer.SoapMockEndpoint(ctx, "Pastries Service", "1.0")
	require.NoError(t, err)
	require.Equal(t, endpoint+"/soap/Pastries Service/1.0", baseApiUrl)

	baseApiUrl, err = microcksContainer.RestMockEndpoint(ctx, "API Pastries", "0.0.1")
	require.NoError(t, err)
	require.Equal(t, endpoint+"/rest/API Pastries/0.0.1", baseApiUrl)

	baseApiUrl, err = microcksContainer.GraphQLMockEndpoint(ctx, "Pastries Graph", "1")
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

// MicrocksMockingFunctionality tests the Microcks mocking functionality.
func MicrocksMockingFunctionality(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	baseApiUrl, err := microcksContainer.RestMockEndpoint(ctx, "API Pastries", "0.0.1")
	require.NoError(t, err)

	resp, err := http.Get(baseApiUrl + "/pastries/Millefeuille")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Unmarshal body using a generic interface.
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	var pastry = map[string]string{}
	json.Unmarshal([]byte(body), &pastry)

	require.Equal(t, "Millefeuille", pastry["name"])

	// Check it has been called once.
	called, err := microcksContainer.Verify(ctx, "API Pastries", "0.0.1")
	require.NoError(t, err)
	require.True(t, called)

	callCount, err := microcksContainer.ServiceInvocationsCount(ctx, "API Pastries", "0.0.1")
	require.NoError(t, err)
	require.Equal(t, 1, callCount)

	// Check that mock from secondary artifact has been loaded.
	resp, err = http.Get(baseApiUrl + "/pastries/Eclair Chocolat")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.StatusCode)

	// Unmarshal body using a generic interface.
	body, err = io.ReadAll(resp.Body)
	require.NoError(t, err)

	pastry = map[string]string{}
	json.Unmarshal([]byte(body), &pastry)

	require.Equal(t, "Eclair Chocolat", pastry["name"])

	// Check it has been called a second time.
	callCount, err = microcksContainer.ServiceInvocationsCount(ctx, "API Pastries", "0.0.1")
	require.NoError(t, err)
	require.Equal(t, 2, callCount)
}

// MicrocksAsyncMockingFunctionality tests the Microcks async mocking functionality.
func MicrocksAsyncMockingFunctionality(t *testing.T, ctx context.Context, microcksAsyncMinionContainer *async.MicrocksAsyncMinionContainer) {
	wsEndpoint, err := microcksAsyncMinionContainer.WSMockEndpoint(ctx, "Pastry orders API", "0.1.0", "SUBSCRIBE pastry/orders")
	if err != nil {
		require.NoError(t, err)
		return
	}
	expectedMessage := "{\"id\":\"4dab240d-7847-4e25-8ef3-1530687650c8\",\"customerId\":\"fe1088b3-9f30-4dc1-a93d-7b74f0a072b9\",\"status\":\"VALIDATED\",\"productQuantities\":[{\"quantity\":2,\"pastryName\":\"Croissant\"},{\"quantity\":1,\"pastryName\":\"Millefeuille\"}]}"

	// Check signals.
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt)

	// Connect to websocket.
	c, _, err := websocket.DefaultDialer.Dial(wsEndpoint, nil)
	if err != nil {
		require.NoError(t, err)
		return
	}
	defer c.Close()

	// Receive messages.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			_, message, err := c.ReadMessage()
			if err != nil {
				return
			}
			require.Equal(t, expectedMessage, string(message))
		}
	}()

	for {
		select {
		// Wait 7 seconds for messages from Async Minion WebSocket to get at least 2 messages.
		case <-time.After(7 * time.Second):
			return
		case <-done:
			return
		case <-interrupt:
			// Cleanly close the connection.
			err := c.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
			if err != nil {
				return
			}
			select {
			case <-done:
			case <-time.After(time.Second):
			}
			return
		}
	}
}

// MicrocksAsyncKafkaMockingFunctionality tests the Microcks async Kafka mocking functionality.
func MicrocksAsyncKafkaMockingFunctionality(t *testing.T, ctx context.Context, kafkaContainer *kafkaTC.KafkaContainer, microcksAsyncMinionContainer *async.MicrocksAsyncMinionContainer) {
	kafkaTopic := microcksAsyncMinionContainer.KafkaMockTopic("Pastry orders API", "0.1.0", "SUBSCRIBE pastry/orders")
	expectedMessage := "{\"id\":\"4dab240d-7847-4e25-8ef3-1530687650c8\",\"customerId\":\"fe1088b3-9f30-4dc1-a93d-7b74f0a072b9\",\"status\":\"VALIDATED\",\"productQuantities\":[{\"quantity\":2,\"pastryName\":\"Croissant\"},{\"quantity\":1,\"pastryName\":\"Millefeuille\"}]}"

	brokers, err := kafkaContainer.Brokers(ctx)
	if err != nil {
		require.NoError(t, err)
		return
	}

	randomID := fmt.Sprintf("random-%d", time.Now().UnixMilli())
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  brokers[0],
		"group.id":           randomID,
		"client.id":          randomID,
		"auto.offset.reset":  "latest",
		"enable.auto.commit": false,
	})
	if err != nil {
		require.NoError(t, err)
		return
	}
	defer c.Close()

	if err := c.Subscribe(kafkaTopic, nil); err != nil {
		require.NoError(t, err)
		return
	}

	// Receive messages.
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			message, err := c.ReadMessage(time.Second)
			if err != nil {
				return
			}
			require.Equal(t, expectedMessage, message.String())
		}
	}()

	for {
		select {
		// Wait 7 seconds for messages from Async Minion Kafka to get at least 2 messages.
		case <-time.After(7 * time.Second):
			return
		case <-done:
			return
		}
	}
}

// AssertBadImplementation helps to assert the endpoint with a bad implementation.
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

	// Retrieve messages for the failing test case.
	messages, err := microcksContainer.MessagesForTestCase(ctx, testResult, "GET /pastries")
	require.NoError(t, err)
	require.NotNil(t, messages)
	require.Equal(t, 3, len(*messages))
	for _, m := range *messages {
		require.NotNil(t, m.Request)
		require.NotNil(t, m.Response)
		require.NotEmpty(t, *m.Response.Content)
	}
}

// AssertGoodImplementation helps to assert the endpoint with a good implementation.
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

// MicrocksContractTestingFunctionality helps to assert contract testing functionality.
func MicrocksContractTestingFunctionality(
	t *testing.T,
	ctx context.Context,
	mc *microcks.MicrocksContainer,
	badImpl,
	goodImpl testcontainers.Container) {

	// Bad implementation.
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

	// Check nil first step result message.
	t0bad := (*testResultBad.TestCaseResults)[0].TestStepResults
	require.NotNil(t, t0bad)
	s0bad := (*t0bad)[0]
	require.NotNil(t, s0bad)
	require.True(t, strings.Contains(*s0bad.Message, "Valid"), "Message not contain Valid word", *s0bad.Message)

	// Good implementation.
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

	// Check nil first step result message.
	t0good := (*testResultGood.TestCaseResults)[0].TestStepResults
	require.NotNil(t, t0good)
	s0good := (*t0good)[0]
	require.NotNil(t, s0good)
	require.Nil(t, s0good.Message)
}

// Deprecated: use testcontainers.WithNetwork once it's released.
// WithNetwork is a custom request option that adds a network to a container.
// This is a temporary option until the next release of testcontainers-go, which will include
// this option.
func WithNetwork(network string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Networks = []string{network}

		return nil
	}
}

// PrintMicrocksContainerLogs prints the Microcks container logs.
func PrintMicrocksContainerLogs(t *testing.T, ctx context.Context, microcksContainer *microcks.MicrocksContainer) {
	readCloser, err := microcksContainer.Logs(ctx)
	require.NoError(t, err)

	// Example to read data.
	buf := new(bytes.Buffer)
	numOfByte, err := buf.ReadFrom(readCloser)
	require.NoError(t, err)

	readCloser.Close()
	t.Logf("Read: %d bytes, content is: %s", numOfByte, buf.String())
}
