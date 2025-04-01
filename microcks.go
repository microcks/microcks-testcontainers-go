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
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	client "microcks.io/go-client"
)

const (
	DefaultImage = "quay.io/microcks/microcks-uber:latest"

	// DefaultHttpPort represents the default Microcks HTTP port.
	DefaultHttpPort = "8080/tcp"

	// DefaultGrpcPort represents the default Microcks GRPC port.
	DefaultGrpcPort = "9090/tcp"

	// DefaultNetworkAlias represents the default network alias of the the MicrocksContainer.
	DefaultNetworkAlias = "microcks"
)

// MicrocksContainer represents the Microcks container type used in the module.
type MicrocksContainer struct {
	testcontainers.Container
}

// Deprecated: use Run instead
// RunContainer creates an instance of the MicrocksContainer type.
func RunContainer(ctx context.Context, opts ...testcontainers.ContainerCustomizer) (*MicrocksContainer, error) {
	return Run(ctx, DefaultImage, opts...)
}

// Run creates an instance of the MicrocksContainer type.
func Run(ctx context.Context, image string, opts ...testcontainers.ContainerCustomizer) (*MicrocksContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        image,
		ExposedPorts: []string{DefaultHttpPort, DefaultGrpcPort},
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

// WithMainArtifact provides paths to artifacts that will be imported as main or main
// ones within the Microcks container.
// Once it will be started and healthy.
func WithMainArtifact(artifactFilePath string) testcontainers.CustomizeRequestOption {
	return WithArtifact(artifactFilePath, true)
}

// WithSecondaryArtifact provides paths to artifacts that will be imported as main or main
// ones within the Microcks container.
// Once it will be started and healthy.
func WithSecondaryArtifact(artifactFilePath string) testcontainers.CustomizeRequestOption {
	return WithArtifact(artifactFilePath, false)
}

// WithArtifact provides paths to artifacts that will be imported within the Microcks container.
// Once it will be started and healthy.
func WithArtifact(artifactFilePath string, main bool) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		hooks := testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				importArtifactHook(artifactFilePath, main),
			},
		}
		req.LifecycleHooks = append(req.LifecycleHooks, hooks)

		return nil
	}
}

// WithNetwork allows to add a custom network.
// Deprecated: Use network.WithNetwork from testcontainers instead.
func WithNetwork(networkName string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.Networks = append(req.Networks, networkName)
		return nil
	}
}

// WithNetworkAlias allows to add a custom network alias for a specific network.
// Deprecated: Use network.WithNetwork from testcontainers instead.
func WithNetworkAlias(networkName, networkAlias string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.NetworkAliases == nil {
			req.NetworkAliases = make(map[string][]string)
		}
		req.NetworkAliases[networkName] = []string{networkAlias}

		return nil
	}
}

// WithEnv allows to add an environment variable.
func WithEnv(key, value string) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		if req.Env == nil {
			req.Env = make(map[string]string)
		}
		req.Env[key] = value

		return nil
	}
}

// WithHostAccessPorts allows to set the host access ports.
func WithHostAccessPorts(hostAccessPorts []int) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		req.HostAccessPorts = hostAccessPorts

		return nil
	}
}

// WithSecret allows to add a new secret.
func WithSecret(s client.Secret) testcontainers.CustomizeRequestOption {
	return func(req *testcontainers.GenericContainerRequest) error {
		hooks := testcontainers.ContainerLifecycleHooks{
			PostReadies: []testcontainers.ContainerHook{
				createSecretHook(s),
			},
		}
		req.LifecycleHooks = append(req.LifecycleHooks, hooks)

		return nil
	}
}

// HttpEndpoint allows retrieving the Http endpoint where Microcks can be accessed.
// (you'd have to append '/api' to access APIs)
func (container *MicrocksContainer) HttpEndpoint(ctx context.Context) (string, error) {
	ip, err := container.Host(ctx)
	if err != nil {
		return "", err
	}

	port, err := container.MappedPort(ctx, DefaultHttpPort)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("http://%s:%s", ip, port.Port()), nil
}

// SoapMockEndpoint get the exposed mock endpoint for a SOAP Service.
func (container *MicrocksContainer) SoapMockEndpoint(ctx context.Context, service string, version string) (string, error) {
	endpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/soap/%s/%s", endpoint, service, version), nil
}

// SoapMockEndpointPath get the exposed mock endpoint path for a SOAP Service.
func (container *MicrocksContainer) SoapMockEndpointPath(ctx context.Context, service string, version string) string {
	return fmt.Sprintf("/soap/%s/%s", service, version)
}

// ValidatingSoapMockEndpoint get the exposed mock endpoint - with request validation enabled - for a SOAP Service.
func (container *MicrocksContainer) ValidatingSoapMockEndpoint(ctx context.Context, service string, version string) (string, error) {
	endpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/soap/%s/%s?validate=true", endpoint, service, version), nil
}

// ValidatingSoapMockEndpointPath get the exposed mock endpoint path - with request validation enabled - for a SOAP Service.
func (container *MicrocksContainer) ValidatingSoapMockEndpointPath(ctx context.Context, service string, version string) string {
	return fmt.Sprintf("/soap/%s/%s?validate=true", service, version)
}

// RestMockEndpoint get the exposed mock endpoint for a REST Service.
func (container *MicrocksContainer) RestMockEndpoint(ctx context.Context, service string, version string) (string, error) {
	endpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/rest/%s/%s", endpoint, service, version), nil
}

// RestMockEndpointPath get the exposed mock endpoint path for a REST Service.
func (container *MicrocksContainer) RestMockEndpointPath(ctx context.Context, service string, version string) string {
	return fmt.Sprintf("/rest/%s/%s", service, version)
}

// ValidatingRestMockEndpoint get the exposed mock endpoint - with request validation enabled - for a REST Service.
func (container *MicrocksContainer) ValidatingRestMockEndpoint(ctx context.Context, service string, version string) (string, error) {
	endpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/rest-valid/%s/%s", endpoint, service, version), nil
}

// ValidatingRestMockEndpointPath get the exposed mock endpoint path - with request validation enabled - for a REST Service.
func (container *MicrocksContainer) ValidatingRestMockEndpointPath(ctx context.Context, service string, version string) string {
	return fmt.Sprintf("/rest-valid/%s/%s", service, version)
}

// GraphQLMockEndpoint get the exposed mock endpoints for a GraphQL Service.
func (container *MicrocksContainer) GraphQLMockEndpoint(ctx context.Context, service string, version string) (string, error) {
	endpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/graphql/%s/%s", endpoint, service, version), nil
}

// GraphQLMockEndpointPath get the exposed mock endpoints path for a GraphQL Service.
func (container *MicrocksContainer) GraphQLMockEndpointPath(ctx context.Context, service string, version string) string {
	return fmt.Sprintf("/graphql/%s/%s", service, version)
}

// GrpcMockEndpoint get the exposed mock endpoint for a GRPC Service.
func (container *MicrocksContainer) GrpcMockEndpoint(ctx context.Context) (string, error) {
	ip, err := container.Host(ctx)
	if err != nil {
		return "", err
	}

	port, err := container.MappedPort(ctx, DefaultGrpcPort)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("grpc://%s:%s", ip, port.Port()), nil
}

// ImportAsMainArtifact imports an artifact as a primary or main one within the Microcks container.
func (container *MicrocksContainer) ImportAsMainArtifact(ctx context.Context, artifactFilePath string) (int, error) {
	return container.importArtifact(ctx, artifactFilePath, true)
}

// ImportAsSecondaryArtifact imports an artifact as a secondary one within the Microcks container.
func (container *MicrocksContainer) ImportAsSecondaryArtifact(ctx context.Context, artifactFilePath string) (int, error) {
	return container.importArtifact(ctx, artifactFilePath, false)
}

// TestEndpoint launches a conformance test on an endpoint.
func (container *MicrocksContainer) TestEndpoint(ctx context.Context, testRequest *client.TestRequest) (*client.TestResult, error) {
	// Retrieve API endpoint.
	httpEndpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Microcks API endpoint: %w", err)
	}

	// Create Microcks client.
	c, err := client.NewClientWithResponses(httpEndpoint + "/api")
	if err != nil {
		return nil, fmt.Errorf("error creating Microcks client: %w", err)
	}

	testResult, err := c.CreateTestWithResponse(ctx, *testRequest)
	if err != nil {
		return nil, fmt.Errorf("error creating test with response: %w", err)
	}

	if testResult.HTTPResponse.StatusCode == 201 {
		// Retrieve Id and start polling for final result.
		var testResultId string = testResult.JSON201.Id

		// Wait an initial delay to avoid inefficient poll.
		time.Sleep(100 * time.Millisecond)

		// Compute future time that is the end of waiting time frame + extra 1000 to avoid race condition.
		future := nowInMilliseconds() + int64(testRequest.Timeout) + 1000
		for nowInMilliseconds() < future {
			testResultResponse, err := c.GetTestResultWithResponse(ctx, testResultId)
			if err != nil {
				return nil, fmt.Errorf("error getting test result with response: %w", err)
			}

			//fmt.Println(string(testResultResponse.Body[:]))

			// If still in progress, then wait again.
			if testResultResponse.JSON200.InProgress {
				time.Sleep(200 * time.Millisecond)
			} else {
				break
			}
		}

		// Return the final result.
		response, err := c.GetTestResultWithResponse(ctx, testResultId)
		return response.JSON200, err
	}
	return nil, fmt.Errorf("couldn't launch on new test on Microcks. Please check Microcks container logs")
}

// TestEndpointAsync launches a conformance test on an endpoint and will provide result via a channel.
func (container *MicrocksContainer) TestEndpointAsync(ctx context.Context, testRequest *client.TestRequest, testResult chan *client.TestResult) error {
	result, err := container.TestEndpoint(ctx, testRequest)
	if err != nil {
		return fmt.Errorf("error executing TestEndpoint: %w", err)
	}

	testResult <- result // result to channel testResult.

	return nil
}

// MessagesForTestCase retrieves messages exchanged during a test on an endpoint.
func (container *MicrocksContainer) MessagesForTestCase(ctx context.Context, testResult *client.TestResult, operationName string) (*[]client.RequestResponsePair, error) {
	// Retrieve API endpoint.
	httpEndpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Microcks API endpoint: %w", err)
	}

	// Create Microcks client.
	c, err := client.NewClientWithResponses(httpEndpoint + "/api")
	if err != nil {
		return nil, fmt.Errorf("error creating Microcks client: %w", err)
	}

	// Build the test case identifier and call api.
	operation := encodeOperationName(operationName)
	testCaseId := fmt.Sprintf("%s-%s-%s", testResult.Id, strconv.Itoa(int(testResult.TestNumber)), operation)

	response, err := c.GetMessagesByTestCaseWithResponse(ctx, testResult.Id, testCaseId)
	return response.JSON200, err
}

// EventMessagesForTestCase retrieves event messages received during a test on an endpoint.
func (container *MicrocksContainer) EventMessagesForTestCase(ctx context.Context, testResult *client.TestResult, operationName string) (*[]client.UnidirectionalEvent, error) {
	// Retrieve API endpoint.
	httpEndpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return nil, fmt.Errorf("error retrieving Microcks API endpoint: %w", err)
	}

	// Create Microcks client.
	c, err := client.NewClientWithResponses(httpEndpoint + "/api")
	if err != nil {
		return nil, fmt.Errorf("error creating Microcks client: %w", err)
	}

	// Build the test case identifier and call api.
	operation := encodeOperationName(operationName)
	testCaseId := fmt.Sprintf("%s-%s-%s", testResult.Id, strconv.Itoa(int(testResult.TestNumber)), operation)

	response, err := c.GetEventsByTestCaseWithResponse(ctx, testResult.Id, testCaseId)
	return response.JSON200, err
}

// Verify checks that given Service has been invoked at least one time, for the current invocations' date.
func (container *MicrocksContainer) Verify(ctx context.Context, serviceName string, serviceVersion string) (bool, error) {
	return container.VerifyAtDate(ctx, serviceName, serviceVersion, time.Now())
}

// VerifyAtDate checks that given Service has been invoked at least one time, for the given invocations' date.
func (container *MicrocksContainer) VerifyAtDate(ctx context.Context, serviceName string, serviceVersion string, date time.Time) (bool, error) {
	invocationsCount, err := container.ServiceInvocationsCount(ctx, serviceName, serviceVersion)
	if err != nil {
		return false, fmt.Errorf("cannot retrieve invocations stats: %w", err)
	}

	return invocationsCount > 0, nil
}

// ServiceInvocationsCount gets the invocations' count for a given service, identified by its name and version, for the current date.
func (container *MicrocksContainer) ServiceInvocationsCount(ctx context.Context, serviceName string, serviceVersion string) (int, error) {
	// Retrieve API endpoint.
	return container.ServiceInvocationsCountAtDate(ctx, serviceName, serviceVersion, time.Now())
}

// ServiceInvocationsCountAtDate gets the invocations' count for a given service, identified by its name and version, for the given invocations' date.
func (container *MicrocksContainer) ServiceInvocationsCountAtDate(ctx context.Context, serviceName string, serviceVersion string, date time.Time) (int, error) {
	// Retrieve API endpoint.
	httpEndpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return 0, fmt.Errorf("error retrieving Microcks API endpoint: %w", err)
	}

	// Create Microcks client.
	c, err := client.NewClientWithResponses(httpEndpoint + "/api")
	if err != nil {
		return 0, fmt.Errorf("error creating Microcks client: %w", err)
	}

	// Build the day.
	day := formatDay(date)

	// To avoid race condition issue while Microcks server is processing metrics asynchronously.
	// The wait time is lower on other language bindings (100ms). Don't know why...
	time.Sleep(250 * time.Millisecond)
	stats, err := c.GetInvocationStatsByServiceWithResponse(ctx, serviceName, serviceVersion, &client.GetInvocationStatsByServiceParams{
		Day: &day,
	})

	invocationStats := stats.JSON200
	if invocationStats != nil {
		return int(stats.JSON200.DailyCount), err
	}
	return 0, err
}

func importArtifactHook(artifactFilePath string, mainArtifact bool) testcontainers.ContainerHook {
	return func(ctx context.Context, container testcontainers.Container) error {
		microcksContainer := &MicrocksContainer{Container: container}
		_, err := microcksContainer.importArtifact(ctx, artifactFilePath, mainArtifact)
		return err
	}
}

func (container *MicrocksContainer) importArtifact(ctx context.Context, artifactFilePath string, mainArtifact bool) (int, error) {
	// Retrieve API endpoint.
	httpEndpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error retrieving Microcks API endpoint: %w", err)
	}

	// Create Microcks client.
	c, err := client.NewClientWithResponses(httpEndpoint + "/api")
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error creating Microcks client: %w", err)
	}

	// Ensure file exists on fs.
	file, err := os.Open(artifactFilePath)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error opening artifact file: %w", err)
	}
	defer file.Close()

	// Create a multipart request body, reading the file.
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(artifactFilePath))
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error creating multipart form: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error copying file to multipart form: %w", err)
	}

	// Add the mainArtifact flag to request.
	_ = writer.WriteField("mainArtifact", strconv.FormatBool(mainArtifact))
	err = writer.Close()
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error closing multipart form: %w", err)
	}

	response, err := c.UploadArtifactWithBody(ctx, nil, writer.FormDataContentType(), body)
	if err != nil {
		return 0, err
	}
	return response.StatusCode, err
}

func createSecretHook(s client.Secret) testcontainers.ContainerHook {
	return func(ctx context.Context, container testcontainers.Container) error {
		microcksContainer := &MicrocksContainer{Container: container}
		_, err := microcksContainer.createSecret(ctx, s)
		return err
	}
}

func (container *MicrocksContainer) createSecret(ctx context.Context, s client.Secret) (int, error) {
	// Retrieve API endpoint.
	httpEndpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error retrieving Microcks API endpoint: %w", err)
	}

	// Create Microcks client.
	c, err := client.NewClientWithResponses(httpEndpoint + "/api")
	if err != nil {
		return http.StatusInternalServerError, fmt.Errorf("error creating Microcks client: %w", err)
	}

	// Create secret.
	response, err := c.CreateSecret(ctx, s, nil)
	if err != nil {
		return 0, err
	}
	return response.StatusCode, err
}

func nowInMilliseconds() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}

func encodeOperationName(operationName string) string {
	return strings.ReplaceAll(operationName, "/", "!")
}

func formatDay(date time.Time) string {
	return date.Format("20060102")
}
