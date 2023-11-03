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
	"log"
	"mime/multipart"
	"os"
	"path/filepath"
	"strconv"

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

func (container *MicrocksContainer) HttpEndpoint(ctx context.Context) string {
	ip, _ := container.Host(ctx)
	port, _ := container.MappedPort(ctx, defaultHttpPort)
	return fmt.Sprintf("http://%s:%s", ip, port.Port())
}

func (container *MicrocksContainer) RestMockEndpoint(ctx context.Context, service string, version string) string {
	return fmt.Sprintf("%s/rest/%s/%s", container.HttpEndpoint(ctx), service, version)
}

func (container *MicrocksContainer) ImportAsMainArtifact(artifactFilePath string) (int, error) {
	return container.importArtifact(artifactFilePath, true)
}

func (container *MicrocksContainer) ImportAsSecondaryArtifact(artifactFilePath string) (int, error) {
	return container.importArtifact(artifactFilePath, false)
}

func (container *MicrocksContainer) TestEndpoint() {

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
