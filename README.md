# Microcks Testcontainers Go

Go library for Testcontainers that enables embedding Microcks into your Go unit tests with lightweight, throwaway instance thanks to containers

[![License](https://img.shields.io/github/license/microcks/microcks-testcontainers-java?style=for-the-badge&logo=apache)](https://www.apache.org/licenses/LICENSE-2.0)
[![Project Chat](https://img.shields.io/badge/chat-on_zulip-pink.svg?color=ff69b4&style=for-the-badge&logo=zulip)](https://microcksio.zulipchat.com/)
![Go version](https://img.shields.io/github/go-mod/go-version/microcks/microcks-testcontainers-go?style=for-the-badge&logo=go)
![GitHub release](https://img.shields.io/github/downloads-pre/microcks/microcks-testcontainers-go/latest/total?style=for-the-badge)

## Build Status

Latest released version is `v0.1.0`.

Current development version is `v0.2.0`.

## How to use it?

### Include it into your project dependencies

To get the latest version, use go1.21+ and fetch using the `go get` command. For example:

```
go get microcks.io/testcontainers-go@latest
```

To get a specific version, use go1.21+ and fetch the desired version using the `go get` command. For example:

```
go get microcks.io/testcontainers-go@v0.1.0
```

### Startup the container

You just have to specify the container image you'd like to use. This library requires a Microcks `uber` distribution (with no MongoDB dependency).

```go
import (
    microcks "microcks.io/testcontainers-go"
)

microcksContainer, err := microcks.RunContainer(ctx, testcontainers.WithImage("quay.io/microcks/microcks-uber:nightly"))
```

### Import content in Microcks

To use Microcks mocks or contract-testing features, you first need to import OpenAPI, Postman Collection, GraphQL or gRPC artifacts. 
Artifacts can be imported as main/Primary ones or as secondary ones. See [Multi-artifacts support](https://microcks.io/documentation/using/importers/#multi-artifacts-support) for details.

You have do it once the container is running:

```go
status, err := microcksContainer.ImportAsMainArtifact("testdata/apipastries-openapi.yaml")
if err != nil {
    log.Fatal(err)
}

status, err = microcksContainer.ImportAsSecondaryArtifact("testdata/apipastries-postman-collection.json")
if err != nil {
    log.Fatal(err)
}
```

`status` if the status of the Http response from the microcks container and should be equal to `201` in case of success.

Please refer to our [microcks_test](https://github.com/microcks/microcks-testcontainers-go/blob/microcks_test.go) for comprehensive example on how to use it.

### Using mock endpoints for your dependencies

During your test setup, you'd probably need to retrieve mock endpoints provided by Microcks containers to 
setup your base API url calls. You can do it like this:

```go
baseApiUrl := microcksContainer.RestMockEndpoint(ctx, "API Pastries", "0.0.1")
```

The container provides methods for different supported API styles/protocols (Soap, GraphQL, gRPC,...).

The container also provides `HttpEndpoint()` for raw access to those API endpoints.

### Launching new contract-tests

If you want to ensure that your application under test is conformant to an OpenAPI contract (or many contracts),
you can launch a Microcks contract/conformance test using the local server port you're actually running:

```go
import (
    client "microcks.io/go-client"
    microcks "microcks.io/testcontainers-go"
)

// Build a new TestRequest.
testRequest := client.TestRequest{
    ServiceId:    "API Pastries:0.0.1",
    RunnerType:   client.TestRunnerTypeOPENAPISCHEMA,
    TestEndpoint: "http://bad-impl:3001",
    Timeout:      2000,
}

testResult, err := microcksContainer.TestEndpoint(context.Background(), &testRequest)
require.NoError(t, err)

require.False(t, testResult.Success)
require.Equal(t, "http://bad-impl:3001", testResult.TestedEndpoint)
```

The `testResult` gives you access to all details regarding success of failure on different test cases.