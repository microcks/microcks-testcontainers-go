package ensemble_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	kafkaTC "github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/modules/localstack"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"microcks.io/go-client"
	"microcks.io/testcontainers-go/ensemble"
	"microcks.io/testcontainers-go/ensemble/async/connection/kafka"
	"microcks.io/testcontainers-go/internal/test"
)

func TestMockingFunctionalityAtStartup(t *testing.T) {
	ctx := context.Background()

	// Ensemble containers.
	ec, err := ensemble.RunContainers(ctx,
		ensemble.WithMainArtifact("../testdata/apipastries-openapi.yaml"),
		ensemble.WithSecondaryArtifact("../testdata/apipastries-postman-collection.json"),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
	test.MockEndpoints(t, ctx, ec.GetMicrocksContainer())
	test.MicrocksMockingFunctionality(t, ctx, ec.GetMicrocksContainer())
}

func TestPostmanContractTestingFunctionality(t *testing.T) {
	ctx := context.Background()

	// Ensemble containers.
	ec, err := ensemble.RunContainers(
		ctx,
		ensemble.WithMainArtifact("../testdata/apipastries-openapi.yaml"),
		ensemble.WithSecondaryArtifact("../testdata/apipastries-postman-collection.json"),
		ensemble.WithPostman(true),
	)
	require.NoError(t, err)
	networkName := ec.GetNetwork().Name

	// Demo pastry bad implementation.
	badImpl, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:    "quay.io/microcks/contract-testing-demo:02",
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"bad-impl"},
			},
			WaitingFor: wait.ForLog("Example app listening on port 3002"),
		},
		Started: true,
	})
	require.NoError(t, err)

	// Demo pastry good implementation.
	goodImpl, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:    "quay.io/microcks/contract-testing-demo:03",
			Networks: []string{networkName},
			NetworkAliases: map[string][]string{
				networkName: {"good-impl"},
			},
			WaitingFor: wait.ForLog("Example app listening on port 3003"),
		},
		Started: true,
	})
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.GetMicrocksContainer().Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
		if err := badImpl.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
		if err := goodImpl.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
	test.MicrocksContractTestingFunctionality(
		t,
		ctx,
		ec.GetMicrocksContainer(),
		badImpl,
		goodImpl,
	)
}

func TestAsyncFeatureSetup(t *testing.T) {
	ctx := context.Background()

	// Ensemble containers.
	ec, err := ensemble.RunContainers(
		ctx,
		ensemble.WithAsyncFeature(),
		ensemble.WithHostAccessPorts([]int{8080}),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
}

func TestAsyncFeatureMockingFunctionality(t *testing.T) {
	ctx := context.Background()

	// Ensemble containers.
	ec, err := ensemble.RunContainers(
		ctx,
		ensemble.WithAsyncFeature(),
		ensemble.WithMainArtifact("../testdata/pastry-orders-asyncapi.yaml"),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
	test.MicrocksAsyncMockingFunctionality(t, ctx, ec.GetAsyncMinionContainer())
}

func TestAsyncKafkaMockingFunctionality(t *testing.T) {
	ctx := context.Background()

	// Common network.
	net, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		require.NoError(t, err)
		return
	}

	// Kafka container.
	kc, err := kafkaTC.RunContainer(ctx,
		testcontainers.WithImage("confluentinc/confluent-local:7.5.0"),
		network.WithNetwork([]string{"kafka"}, net),
	)
	if err != nil {
		require.NoError(t, err)
		return
	}
	brokers, err := kc.Brokers(ctx)
	if err != nil {
		require.NoError(t, err)
		return
	}

	// Ensemble containers.
	ec, err := ensemble.RunContainers(
		ctx,
		ensemble.WithAsyncFeature(),
		ensemble.WithMainArtifact("../testdata/pastry-orders-asyncapi.yaml"),
		ensemble.WithKafkaConnection(kafka.Connection{
			BootstrapServers: brokers[0],
		}),
		ensemble.WithNetwork(net),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
		if err := kc.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate Kafka container: %s", err)
		}
	})

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
	test.MicrocksAsyncKafkaMockingFunctionality(
		t,
		ctx,
		kc,
		ec.GetAsyncMinionContainer(),
	)
}

func TestAsyncFeatureAmazonSQSTestingFunctionality(t *testing.T) {
	ctx := context.Background()

	// Common network.
	net, err := network.New(ctx, network.WithCheckDuplicate())
	if err != nil {
		require.NoError(t, err)
		return
	}

	// Localstack container.
	lc, err := localstack.RunContainer(ctx,
		testcontainers.WithImage("localstack/localstack:latest"),
		network.WithNetwork([]string{"localstack"}, net),
		testcontainers.WithEnv(map[string]string{
			"SERVICES": "sqs",
		}),
	)
	if err != nil {
		require.NoError(t, err)
		return
	}

	// Ensemble containers.
	tmp := ""
	ec, err := ensemble.RunContainers(
		ctx,
		ensemble.WithAsyncFeature(),
		ensemble.WithMainArtifact("../testdata/pastry-orders-asyncapi.yaml"),
		ensemble.WithNetwork(net),
		ensemble.WithSecret(client.Secret{
			Name:     "localstack secret",
			Username: &tmp, // Missing
			Password: &tmp, // Missing
		}),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
		if err := lc.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate localstack container: %s", err)
		}
	})

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
	test.MicrocksAsyncAmazonSQSContractTestingFunctionality(
		t,
		ctx,
		lc,
		ec.GetAsyncMinionContainer(),
	)
}
