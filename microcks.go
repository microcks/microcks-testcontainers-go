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
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	client "microcks.io/go-client"
)

const (
	defaultImage    = "quay.io/microcks/microcks-uber:latest"
	DefaultHttpPort = "8080/tcp"
	DefaultGrpcPort = "9090/tcp"
)

type MicrocksContainer struct {
	testcontainers.Container
}

// RunContainer creates an instance of the MicrocksContainer type.
func RunContainer(ctx context.Context, opts ...testcontainers.ContainerCustomizer) (*MicrocksContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        defaultImage,
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

// HttpEndpoint allows retrieving the Http endpoint where Microcks can be accessed
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

// RestMockEndpoints get the exposed mock endpoint for a REST Service.
func (container *MicrocksContainer) RestMockEndpoint(ctx context.Context, service string, version string) (string, error) {
	endpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/rest/%s/%s", endpoint, service, version), nil
}

// GraphQLMockEndpoint get the exposed mock endpoints for a GraphQL Service.
func (container *MicrocksContainer) GrapQLMockEndpoint(ctx context.Context, service string, version string) (string, error) {
	endpoint, err := container.HttpEndpoint(ctx)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s/graphql/%s/%s", endpoint, service, version), nil
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

		// Compute future time that is the end of waiting timeframe.
		future := nowInMilliseconds() + int64(testRequest.Timeout)
		for nowInMilliseconds() < future {
			testResultResponse, err := c.GetTestResultWithResponse(ctx, testResultId)
			if err != nil {
				return nil, fmt.Errorf("error getting test result with response: %w", err)
			}

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
	return response.StatusCode, err
}

func nowInMilliseconds() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
