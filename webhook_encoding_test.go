package microcks

import "testing"

func TestBuildWebhookOperationId(t *testing.T) {
	tests := []struct {
		name          string
		serviceId     string
		operationName string
		want          string
	}{
		{
			name:          "plain operation name",
			serviceId:     "svc123",
			operationName: "newPet WEBHOOK",
			want:          "svc123-newPet WEBHOOK",
		},
		{
			name:          "hyphens in operation name stay literal",
			serviceId:     "svc123",
			operationName: "order-placed-event WEBHOOK",
			want:          "svc123-order-placed-event WEBHOOK",
		},
		{
			name:          "slashes encoded as bang",
			serviceId:     "svc123",
			operationName: "POST /api/v1/user-profiles",
			want:          "svc123-POST !api!v1!user-profiles",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildWebhookOperationId(tt.serviceId, tt.operationName)
			if got != tt.want {
				t.Fatalf("buildWebhookOperationId(%q, %q) = %q, want %q",
					tt.serviceId, tt.operationName, got, tt.want)
			}
		})
	}
}
