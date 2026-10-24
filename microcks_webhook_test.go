package microcks

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
)

const webhookTestServiceID = "60f123456789abcdef012345"

type webhookRegistrationTestContainer struct {
	testcontainers.Container
	host string
	port nat.Port
}

func (c webhookRegistrationTestContainer) Host(context.Context) (string, error) {
	return c.host, nil
}

func (c webhookRegistrationTestContainer) MappedPort(context.Context, nat.Port) (nat.Port, error) {
	return c.port, nil
}

func TestRegisterWebhookEncodesOperationName(t *testing.T) {
	tests := []struct {
		name          string
		operationName string
		wantOperation string
	}{
		{
			name:          "hyphens are preserved",
			operationName: "order-placed-event WEBHOOK",
			wantOperation: webhookTestServiceID + "-order-placed-event WEBHOOK",
		},
		{
			name:          "slashes are encoded and hyphens are preserved",
			operationName: "POST /pets-store/new-pet",
			wantOperation: webhookTestServiceID + "-POST !pets-store!new-pet",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var receivedOperationID string
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.URL.Path {
				case "/api/services":
					w.Header().Set("Content-Type", "application/json")
					_, _ = io.WriteString(w, `[{"id":"`+webhookTestServiceID+`","name":"Petstore Webhooks","version":"2.0.0"}]`)
				case "/api/webhooks":
					var body struct {
						OperationID string `json:"operationId"`
					}
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					receivedOperationID = body.OperationID
					w.WriteHeader(http.StatusCreated)
				default:
					http.NotFound(w, r)
				}
			}))
			t.Cleanup(server.Close)

			port := server.Listener.Addr().(*net.TCPAddr).Port
			container := &MicrocksContainer{Container: webhookRegistrationTestContainer{
				host: "127.0.0.1",
				port: nat.Port(strconv.Itoa(port) + "/tcp"),
			}}
			status, err := container.registerWebhook(context.Background(), WebhookCoordinates{
				ServiceId:     "Petstore Webhooks:2.0.0",
				OperationName: tt.operationName,
				TargetUrl:     "http://localhost:8080/callback",
			})
			if err != nil {
				t.Fatalf("registerWebhook() error = %v", err)
			}
			if status != http.StatusCreated {
				t.Fatalf("registerWebhook() status = %d, want %d", status, http.StatusCreated)
			}
			if receivedOperationID != tt.wantOperation {
				t.Errorf("registration operationId = %q, want %q", receivedOperationID, tt.wantOperation)
			}
		})
	}
}
