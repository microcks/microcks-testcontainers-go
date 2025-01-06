# Microcks Testcontainers Go

Go library for Testcontainers that enables embedding Microcks into your Go unit tests with lightweight, throwaway instance thanks to containers

![GitHub Workflow Status](https://img.shields.io/github/actions/workflow/status/microcks/microcks-testcontainers-go/build-verify.yml?logo=github&style=for-the-badge)
[![License](https://img.shields.io/github/license/microcks/microcks-testcontainers-java?style=for-the-badge&logo=apache)](https://www.apache.org/licenses/LICENSE-2.0)
[![Project Chat](https://img.shields.io/badge/discord-microcks-pink.svg?color=7289da&style=for-the-badge&logo=discord)](https://microcks.io/discord-invite/)
![Go version](https://img.shields.io/github/go-mod/go-version/microcks/microcks-testcontainers-go?style=for-the-badge&logo=go)
![GitHub release](https://img.shields.io/github/downloads-pre/microcks/microcks-testcontainers-go/latest/total?style=for-the-badge)
[![Artifact HUB](https://img.shields.io/endpoint?url=https://artifacthub.io/badge/repository/microcks-uber-image&style=for-the-badge)](https://artifacthub.io/packages/search?repo=microcks-uber-image)
[![CNCF Landscape](https://img.shields.io/badge/CNCF%20Landscape-5699C6?style=for-the-badge&logo=cncf)](https://landscape.cncf.io/?item=app-definition-and-development--application-definition-image-build--microcks)

## Build Status

Latest released version is `v0.2.1`.

Current development version is `v0.3.0`.

#### Fossa license and security scans

[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fmicrocks%2Fmicrocks-testcontainers-go.svg?type=shield&issueType=license)](https://app.fossa.com/projects/git%2Bgithub.com%2Fmicrocks%2Fmicrocks-testcontainers-go?ref=badge_shield&issueType=license)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fmicrocks%2Fmicrocks-testcontainers-go.svg?type=shield&issueType=security)](https://app.fossa.com/projects/git%2Bgithub.com%2Fmicrocks%2Fmicrocks-testcontainers-go?ref=badge_shield&issueType=security)
[![FOSSA Status](https://app.fossa.com/api/projects/git%2Bgithub.com%2Fmicrocks%2Fmicrocks-testcontainers-go.svg?type=small)](https://app.fossa.com/projects/git%2Bgithub.com%2Fmicrocks%2Fmicrocks-testcontainers-go?ref=badge_small)

#### OpenSSF best practices on Microcks core

[![CII Best Practices](https://bestpractices.coreinfrastructure.org/projects/7513/badge)](https://bestpractices.coreinfrastructure.org/projects/7513)
[![OpenSSF Scorecard](https://api.securityscorecards.dev/projects/github.com/microcks/microcks/badge)](https://securityscorecards.dev/viewer/?uri=github.com/microcks/microcks)

## Community

* [Documentation](https://microcks.io/documentation/tutorials/getting-started/)
* [Microcks Community](https://github.com/microcks/community) and community meeting
* Join us on [Discord](https://microcks.io/discord-invite/), on [GitHub Discussions](https://github.com/orgs/microcks/discussions) or [CNCF Slack #microcks channel](https://cloud-native.slack.com/archives/C05BYHW1TNJ)

To get involved with our community, please make sure you are familiar with the project's [Code of Conduct](./CODE_OF_CONDUCT.md).

## How to use it?

### Include it into your project dependencies

To get the latest version, use go1.21+ and fetch using the `go get` command. For example:

```
go get microcks.io/testcontainers-go@latest
```

To get a specific version, use go1.22+ and fetch the desired version using the `go get` command. For example:

```
go get microcks.io/testcontainers-go@v0.2.0
```

### Startup the container

You just have to specify the container image you'd like to use. This library requires a Microcks `uber` distribution (with no MongoDB dependency).

```go
import (
    microcks "microcks.io/testcontainers-go"
)

microcksContainer, err := microcks.Run(ctx, "quay.io/microcks/microcks-uber:nightly")
```

### Import content in Microcks

To use Microcks mocks or contract-testing features, you first need to import OpenAPI, Postman Collection, GraphQL or gRPC artifacts. 
Artifacts can be imported as main/Primary ones or as secondary ones. See [Multi-artifacts support](https://microcks.io/documentation/using/importers/#multi-artifacts-support) for details.

You can do it before starting the container using simple paths:

```go
import (
    microcks "microcks.io/testcontainers-go"
)

microcksContainer, err := microcks.Run(ctx, 
    "quay.io/microcks/microcks-uber:nightly",
    microcks.WithMainArtifact("testdata/apipastries-openapi.yaml"),
    microcks.WithSecondaryArtifact("testdata/apipastries-postman-collection.json"),
)
```

or once the container started using `ImportAsMainArtifact` and `ImportAsSecondaryArtifact` functions:

```go
status, err := microcksContainer.ImportAsMainArtifact(context.Background(), "testdata/apipastries-openapi.yaml")
if err != nil {
    log.Fatal(err)
}

status, err = microcksContainer.ImportAsSecondaryArtifact(context.Background(), "testdata/apipastries-postman-collection.json")
if err != nil {
    log.Fatal(err)
}
```

`status` if the status of the Http response from the microcks container and should be equal to `201` in case of success.

Please refer to our [microcks_test](https://github.com/microcks/microcks-testcontainers-go/blob/main/microcks_test.go) for comprehensive example on how to use it.

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

### Advanced features with MicrocksContainersEnsemble

The `MicrocksContainer` referenced above supports essential features of Microcks provided by the main Microcks container.
The list of supported features is the following:

* Mocking of REST APIs using different kinds of artifacts,
* Contract-testing of REST APIs using `OPEN_API_SCHEMA` runner/strategy,
* Mocking and contract-testing of SOAP WebServices,
* Mocking and contract-testing of GraphQL APIs,
* Mocking and contract-testing of gRPC APIs.

To support features like Asynchronous API and `POSTMAN` contract-testing, we introduced `MicrocksContainersEnsemble` that allows managing
additional Microcks services. `MicrocksContainersEnsemble` allow you to implement
[Different levels of API contract testing](https://medium.com/@lbroudoux/different-levels-of-api-contract-testing-with-microcks-ccc0847f8c97)
in the Inner Loop with Testcontainers!

A `MicrocksContainersEnsemble` conforms to Testcontainers lifecycle methods and presents roughly the same interface
as a `MicrocksContainer`. You can create and build an ensemble that way:

```go
import (
    ensemble "microcks.io/testcontainers-go/ensemble"
)

ensembleContainers, err := ensemble.RunContainers(ctx, 
    ensemble.WithMainArtifact("testdata/apipastries-openapi.yaml"),
    ensemble.WithSecondaryArtifact("testdata/apipastries-postman-collection.json"),
)
```

A `MicrocksContainer` is wrapped by an ensemble and is still available to import artifacts and execute test methods.
You have to access it using:

```go
microcks := ensemble.GetMicrocksContainer();
microcks.ImportAsMainArtifact(...);
microcks.Logs(...);
```

Please refer to our [ensemble tests](https://github.com/microcks/microcks-testcontainers-go/blob/main/ensemble/ensemble_test.go) for comprehensive example on how to use it.

#### Postman contract-testing

On this `ensemble` you may want to enable additional features such as Postman contract-testing:

```go
import (
    ensemble "microcks.io/testcontainers-go/ensemble"
)

ensembleContainers, err := ensemble.RunContainers(ctx,
    // Microcks container in ensemble
    ensemble.WithMainArtifact("testdata/apipastries-openapi.yaml"),
    ensemble.WithSecondaryArtifact("testdata/apipastries-postman-collection.json"),

    // Postman container in ensemble
    ensemble.WithPostman(true),
)
```

You can execute a `POSTMAN` test using an ensemble that way:

```go
// Build a new TestRequest.
testRequest := client.TestRequest{
    ServiceId:    "API Pastries:0.0.1",
    RunnerType:   client.TestRunnerTypePOSTMAN,
    TestEndpoint: "http://good-impl:3003",
    Timeout:      2000,
}

testResult := ensemble.
    GetMicrocksContainer().
    TestEndpoint(context.Background(), testRequest);
```

#### Asynchronous API support

Asynchronous API feature need to be explicitly enabled as well. In the case you want to use it for mocking purposes,
you'll have to specify additional connection details to the broker of your choice. See an example below with connection
to a Kafka broker:

```go
ensembleContainers, err := ensemble.RunContainers(ctx,
	// ...
	ensemble.WithAsyncFeature(),
	ensemble.WithKafkaConnection(kafka.Connection{
		BootstrapServers: "kafka:9092",
	}),
)
```

##### Using mock endpoints for your dependencies

Once started, the `ensembleContainers.GetAsyncMinionContainer()` provides methods for retrieving mock endpoint names for the different
supported protocols (WebSocket, Kafka, SQS and SNS).

```go
kafkaTopic := ensembleContainers.
	GetAsyncMinionContainer().
	KafkaMockTopic("Pastry orders API", "0.1.0", "SUBSCRIBE pastry/orders")
```

##### Launching new contract-tests

Using contract-testing techniques on Asynchronous endpoints may require a different style of interacting with the Microcks container. For example, you may need to:
* Start the test making Microcks listen to the target async endpoint,
* Activate your System Under Tests so that it produces an event,
* Finalize the Microcks tests and actually ensure you received one or many well-formed events.

For that the `MicrocksContainer` now provides a `TestEndpointAsync(ctx context.Context, testRequest *client.TestRequest, testResult chan *client.TestResult)` method that actually uses a channel. Once invoked, you may trigger your application events and then wait to receive the future result to assert like this:

```go
// Start the test, making Microcks listen the endpoint provided in testRequest
testResultChan := make(chan *client.TestResult)
go ensembleContainers.GetMicrocksContainer().TestEndpointAsync(ctx, &testRequest, testResultChan)

// Here below: activate your app to make it produce events on this endpoint.
// myapp.invokeBusinessMethodThatTriggerEvents();

// Now retrieve the final test result and assert.
testResult := <-testResultChan
require.NoError(t, err)
require.True(t, testResult.Success)
```