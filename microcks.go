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
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"
	"time"

	client "github.com/microcks/microcks-go-client"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const defaultImage = "quay.io/microcks/microcks-uber:latest"

const defaultHttpPort = "8080/tcp"
const defaultGrpcPort = "9090/tcp"

type MicrocksContainer struct {
	testcontainers.Container
}

// RunContainer creates an instance of the MicrocksContainer type.
func RunContainer(ctx context.Context, opts ...testcontainers.ContainerCustomizer) (*MicrocksContainer, error) {
	req := testcontainers.ContainerRequest{
		Image:        defaultImage,
		ExposedPorts: []string{defaultHttpPort, defaultGrpcPort},
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
func (container *MicrocksContainer) HttpEndpoint(ctx context.Context) string {
	ip, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, defaultHttpPort)
	return fmt.Sprintf("http://%s:%s", ip, port.Port())
}

// SoapMockEndpoint get the exposed mock endpoint for a SOAP Service.
func (container *MicrocksContainer) SoapMockEndpoint(ctx context.Context, service string, version string) string {
	return fmt.Sprintf("%s/soap/%s/%s", container.HttpEndpoint(ctx), service, version)
}

// RestMockEndpoints get the exposed mock endpoint for a REST Service.
func (container *MicrocksContainer) RestMockEndpoint(ctx context.Context, service string, version string) string {
	return fmt.Sprintf("%s/rest/%s/%s", container.HttpEndpoint(ctx), service, version)
}

// GraphQLMockEndpoint get the exposed mock endpoints for a GraphQL Service.
func (container *MicrocksContainer) GrapQLMockEndpoint(ctx context.Context, service string, version string) string {
	return fmt.Sprintf("%s/graphql/%s/%s", container.HttpEndpoint(ctx), service, version)
}

// GrpcMockEndpoint get the exposed mock endpoint for a GRPC Service.
func (container *MicrocksContainer) GrpcMockEndpoint(ctx context.Context) string {
	ip, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, defaultGrpcPort)
	return fmt.Sprintf("grpc://%s:%s", ip, port.Port())
}

// ImportAsMainArtifact imports an artifact as a primary or main one within the Microcks container.
func (container *MicrocksContainer) ImportAsMainArtifact(artifactFilePath string) (int, error) {
	return container.importArtifact(artifactFilePath, true)
}

// ImportAsSecondaryArtifact imports an artifact as a secondary one within the Microcks container.
func (container *MicrocksContainer) ImportAsSecondaryArtifact(artifactFilePath string) (int, error) {
	return container.importArtifact(artifactFilePath, false)
}

// TestEndpoint launches a conformance test on an endpoint.
func (container *MicrocksContainer) TestEndpoint(testRequest *client.TestRequest) (*client.TestResult, error) {
	// Get context and retrieve API endpoint.
	ctx := context.Background()
	httpEndpoint := container.HttpEndpoint(ctx)

	// Create Microcks client.
	c, err := client.NewClientWithResponses(httpEndpoint + "/api")
	if err != nil {
		log.Fatal(err)
	}

	testResult, err := c.CreateTestWithResponse(ctx, *testRequest)
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
				log.Fatal(err)
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
	return nil, errors.New("Couldn't launch on new test on Microcks. Please check Microcks container logs")
}

func (container *MicrocksContainer) importArtifact(artifactFilePath string, mainArtifact bool) (int, error) {
	// Get context and retrieve API endpoint.
	ctx := context.Background()
	httpEndpoint := container.HttpEndpoint(ctx)

	// Create Microcks client.
	c, err := client.NewClientWithResponses(httpEndpoint + "/api")
	if err != nil {
		log.Fatal(err)
	}

	// Ensure file exists on fs.
	file, err := os.Open(artifactFilePath)
	if err != nil {
		log.Fatal(err)
	}
	defer file.Close()

	// Create a multipart request body, reading the file.
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("file", filepath.Base(artifactFilePath))
	if err != nil {
		log.Fatal(err)
	}
	_, err = io.Copy(part, file)
	if err != nil {
		log.Fatal(err)
	}

	// Add the mainArtifact flag to request.
	_ = writer.WriteField("mainArtifact", strconv.FormatBool(mainArtifact))
	err = writer.Close()
	if err != nil {
		log.Fatal(err)
	}

	response, err := c.UploadArtifactWithBody(ctx, nil, writer.FormDataContentType(), body)
	return response.StatusCode, err
}

func nowInMilliseconds() int64 {
	return time.Now().UnixNano() / int64(time.Millisecond)
}
