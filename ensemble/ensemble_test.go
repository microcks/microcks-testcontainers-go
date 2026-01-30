package ensemble_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	pubsub "cloud.google.com/go/pubsub/v2"
	"cloud.google.com/go/pubsub/v2/apiv1/pubsubpb"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	pubsubTC "github.com/testcontainers/testcontainers-go/modules/gcloud/pubsub"
	kafkaTC "github.com/testcontainers/testcontainers-go/modules/kafka"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/api/option"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	microcksClient "microcks.io/go-client"
	"microcks.io/testcontainers-go/ensemble"
	"microcks.io/testcontainers-go/ensemble/async/connection/googlepubsub"
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
		ensemble.WithPostman(),
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
		ensemble.WithDebugLogLevel(),
		ensemble.WithHostAccessPorts([]int{8080}),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate container: %s", err)
		}
	})

	// Checking DEBUG [ in logs.
	readCloser, err := ec.GetAsyncMinionContainer().Logs(ctx)
	require.NoError(t, err)

	buf := new(bytes.Buffer)
	_, err = buf.ReadFrom(readCloser)
	require.NoError(t, err)
	readCloser.Close()

	require.Contains(t, buf.String(), "DEBUG [", "Expected to find 'DEBUG [' log line in Async Minion logs")

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
	kc, err := kafkaTC.Run(ctx,
		"confluentinc/confluent-local:7.5.0",
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

func TestAsyncGooglePubSubMockingFunctionality(t *testing.T) {
	ctx := context.Background()

	// Common network.
	net, err := network.New(ctx, network.WithCheckDuplicate())
	require.NoError(t, err)

	// Start Google PubSub Emulator container.
	emulator, err := pubsubTC.Run(
		ctx,
		"gcr.io/google.com/cloudsdktool/google-cloud-cli:549.0.0-emulators",
		network.WithNetwork([]string{"pubsub-emulator"}, net),
	)
	require.NoError(t, err)

	// Ensemble containers.
	ec, err := ensemble.RunContainers(
		ctx,
		ensemble.WithAsyncFeature(),
		ensemble.WithMainArtifact("../testdata/pastry-orders-asyncapi.yaml"),
		ensemble.WithGooglePubSubConnection(googlepubsub.Connection{
			ProjectId:    "my-custom-project",
			EmulatorHost: "pubsub-emulator:8085",
		}),
		ensemble.WithNetwork(net),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate ensemble: %s", err)
		}
		if err := emulator.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate emulator: %s", err)
		}
	})

	// Get the mock topic name.
	pubSubTopic := ec.GetAsyncMinionContainer().GooglePubSubMockTopic("Pastry orders API", "0.1.0", "SUBSCRIBE pastry/orders")
	require.Equal(t, "PastryordersAPI-0.1.0-pastry-orders", pubSubTopic)

	projectId := "my-custom-project"
	subscriptionId := "my-subscription-id"

	// Wait for minion to create the topic.
	time.Sleep(2 * time.Second)

	// Connect to PubSub emulator.
	conn, err := grpc.NewClient(emulator.URI(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	client, err := pubsub.NewClient(ctx, projectId,
		option.WithGRPCConn(conn),
		option.WithoutAuthentication(),
	)
	require.NoError(t, err)
	defer client.Close()

	println("PuSub topic: " + pubSubTopic)

	// Create subscription to receive mock messages.
	createdSub, err := client.SubscriptionAdminClient.CreateSubscription(ctx, &pubsubpb.Subscription{
		Name:  "projects/my-custom-project/subscriptions/" + subscriptionId,
		Topic: "projects/my-custom-project/topics/" + pubSubTopic,
	})
	require.NoError(t, err)

	// Receive messages.
	var messages []string
	cctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	sub := client.Subscriber(createdSub.Name)
	err = sub.Receive(cctx, func(ctx context.Context, msg *pubsub.Message) {
		messages = append(messages, string(msg.Data))
		msg.Ack()
	})

	// Check that we received messages.
	require.NotEmpty(t, messages)

	// Verify message structure.
	expectedMessage := map[string]interface{}{
		"id":         "4dab240d-7847-4e25-8ef3-1530687650c8",
		"customerId": "fe1088b3-9f30-4dc1-a93d-7b74f0a072b9",
		"status":     "VALIDATED",
		"productQuantities": []interface{}{
			map[string]interface{}{"quantity": float64(2), "pastryName": "Croissant"},
			map[string]interface{}{"quantity": float64(1), "pastryName": "Millefeuille"},
		},
	}

	var actualMessage map[string]interface{}
	err = json.Unmarshal([]byte(messages[0]), &actualMessage)
	require.NoError(t, err)
	require.Equal(t, expectedMessage, actualMessage)

	// Tests & assertions.
	test.ConfigRetrieval(t, ctx, ec.GetMicrocksContainer())
}

func TestAsyncGooglePubSubContractTestingFunctionality(t *testing.T) {
	ctx := context.Background()

	// Common network.
	net, err := network.New(ctx, network.WithCheckDuplicate())
	require.NoError(t, err)

	// Start Google PubSub Emulator container.
	emulator, err := pubsubTC.Run(
		ctx,
		"gcr.io/google.com/cloudsdktool/google-cloud-cli:549.0.0-emulators",
		network.WithNetwork([]string{"pubsub-emulator"}, net),
	)
	require.NoError(t, err)

	// Ensemble containers.
	ec, err := ensemble.RunContainers(
		ctx,
		ensemble.WithAsyncFeature(),
		ensemble.WithMainArtifact("../testdata/pastry-orders-asyncapi.yaml"),
		ensemble.WithNetwork(net),
	)
	require.NoError(t, err)

	// Cleanup containers.
	t.Cleanup(func() {
		if err := ec.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate ensemble: %s", err)
		}
		if err := emulator.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate emulator: %s", err)
		}
	})

	projectId := "my-custom-project"
	topicId := "pastry-orders"

	// Bad message has no status, good message has one.
	badMessage := `{"id":"abcd","customerId":"efgh","productQuantities":[{"quantity":2,"pastryName":"Croissant"},{"quantity":1,"pastryName":"Millefeuille"}]}`
	goodMessage := `{"id":"abcd","customerId":"efgh","status":"CREATED","productQuantities":[{"quantity":2,"pastryName":"Croissant"},{"quantity":1,"pastryName":"Millefeuille"}]}`

	// Connect to PubSub emulator.
	conn, err := grpc.NewClient(emulator.URI(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	require.NoError(t, err)
	defer conn.Close()

	pubsubClient, err := pubsub.NewClient(ctx, projectId,
		option.WithGRPCConn(conn),
		option.WithoutAuthentication(),
	)
	require.NoError(t, err)
	defer pubsubClient.Close()

	// Create the topic.
	topicName := "projects/" + projectId + "/topics/" + topicId
	_, err = pubsubClient.TopicAdminClient.CreateTopic(ctx, &pubsubpb.Topic{
		Name: topicName,
	})
	require.NoError(t, err)

	// Create publisher.
	publisher := pubsubClient.Publisher(topicName)
	defer publisher.Stop()

	// First test should fail with validation failure messages.
	testRequest := microcksClient.TestRequest{
		ServiceId:    "Pastry orders API:0.1.0",
		RunnerType:   microcksClient.TestRunnerTypeASYNCAPISCHEMA,
		TestEndpoint: "googlepubsub://my-custom-project/pastry-orders?emulatorHost=pubsub-emulator:8085",
		Timeout:      3000,
	}

	testResultChan := make(chan *microcksClient.TestResult)
	go func() {
		err = ec.GetMicrocksContainer().TestEndpointAsync(ctx, &testRequest, testResultChan)
		require.NoError(t, err)
	}()

	// Wait a bit to ensure minion is connected.
	time.Sleep(400 * time.Millisecond)

	// Publish bad messages.
	for i := 0; i < 5; i++ {
		result := publisher.Publish(ctx, &pubsub.Message{
			Data: []byte(badMessage),
		})
		_, err := result.Get(ctx)
		require.NoError(t, err)
		t.Logf("Sending bad message %d on Google PubSub topic", i)
		time.Sleep(100 * time.Millisecond)
	}

	// Get test result.
	testResult := <-testResultChan
	require.NotNil(t, testResult)
	require.False(t, testResult.Success)
	require.Equal(t, "googlepubsub://my-custom-project/pastry-orders?emulatorHost=pubsub-emulator:8085", testResult.TestedEndpoint)

	// Ensure we had at least one message.
	require.NotEmpty(t, *testResult.TestCaseResults)
	require.NotEmpty(t, (*testResult.TestCaseResults)[0].TestStepResults)
	testStepResults := (*testResult.TestCaseResults)[0].TestStepResults
	testStepResult := (*testStepResults)[0]
	require.NotNil(t, testStepResult.Message)
	require.Contains(t, *testStepResult.Message, "required property 'status' not found")

	// Second test should succeed without validation failure messages.
	testRequest2 := microcksClient.TestRequest{
		ServiceId:    "Pastry orders API:0.1.0",
		RunnerType:   microcksClient.TestRunnerTypeASYNCAPISCHEMA,
		TestEndpoint: "googlepubsub://my-custom-project/pastry-orders?emulatorHost=pubsub-emulator:8085",
		Timeout:      3000,
	}

	testResultChan2 := make(chan *microcksClient.TestResult)
	go func() {
		err = ec.GetMicrocksContainer().TestEndpointAsync(ctx, &testRequest2, testResultChan2)
		require.NoError(t, err)
	}()

	// Wait a bit to ensure minion is connected.
	time.Sleep(400 * time.Millisecond)

	// Publish good messages.
	for i := 0; i < 5; i++ {
		result := publisher.Publish(ctx, &pubsub.Message{
			Data: []byte(goodMessage),
		})
		_, err := result.Get(ctx)
		require.NoError(t, err)
		t.Logf("Sending good message %d on Google PubSub topic", i)
		time.Sleep(100 * time.Millisecond)
	}

	// Get test result.
	testResult2 := <-testResultChan2
	require.NotNil(t, testResult2)
	require.True(t, testResult2.Success)
	require.Equal(t, "googlepubsub://my-custom-project/pastry-orders?emulatorHost=pubsub-emulator:8085", testResult2.TestedEndpoint)

	// Ensure we had at least one message with no error.
	require.NotEmpty(t, *testResult2.TestCaseResults)
	require.NotEmpty(t, (*testResult2.TestCaseResults)[0].TestStepResults)
	testStepResults2 := (*testResult2.TestCaseResults)[0].TestStepResults
	testStepResult2 := (*testStepResults2)[0]
	require.Nil(t, testStepResult2.Message)
}
