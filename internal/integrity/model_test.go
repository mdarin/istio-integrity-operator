// Тесты для Relational Model
package integrity

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestBuildRelationalModel(t *testing.T) {
	tests := []struct {
		name     string
		services []ServiceRecord
		vss      []VirtualServiceRecord
		gateways []GatewayRecord
		wantErr  bool
	}{
		{
			name: "valid complete model",
			services: []ServiceRecord{
				{Namespace: "default", Name: "web", Host: "web.default.svc.cluster.local", Port: 8080, Protocol: "TCP"},
				{Namespace: "default", Name: "api", Host: "api.default.svc.cluster.local", Port: 9090, Protocol: "TCP"},
			},
			gateways: []GatewayRecord{
				{Namespace: "istio-system", Name: "public-gateway"},
			},
			vss: []VirtualServiceRecord{
				{
					Namespace:        "default",
					Name:             "web-vs",
					GatewayNamespace: "istio-system",
					GatewayName:      "public-gateway",
					Host:             "web.example.com",
					ServiceNamespace: "default",
					ServiceName:      "web",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &RelationalModel{
				Services:        tt.services,
				VirtualServices: tt.vss,
				Gateways:        tt.gateways,
			}

			if len(model.Services) != len(tt.services) {
				t.Errorf("Expected %d services, got %d", len(tt.services), len(model.Services))
			}

			if len(model.VirtualServices) != len(tt.vss) {
				t.Errorf("Expected %d virtual services, got %d", len(tt.vss), len(model.VirtualServices))
			}
		})
	}
}

func TestShouldProcessService(t *testing.T) {
	operator := &SQLiteIntegrityOperator{}

	tests := []struct {
		name        string
		annotations map[string]string
		want        bool
	}{
		{
			name: "service with mesh annotation",
			annotations: map[string]string{
				"mesh.operator.istio.io/managed": "true",
			},
			want: true,
		},
		{
			name:        "service without mesh annotation",
			annotations: map[string]string{},
			want:        false,
		},
		{
			name: "service with other annotations",
			annotations: map[string]string{
				"other.annotation": "value",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &corev1.Service{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: tt.annotations,
				},
			}

			if got := operator.shouldProcessService(svc); got != tt.want {
				t.Errorf("shouldProcessService() = %v, want %v", got, tt.want)
			}
		})
	}
}
