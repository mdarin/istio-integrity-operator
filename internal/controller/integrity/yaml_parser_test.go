// Эти тесты полностью имитируют реальный workflow оператора:

// Чтение YAML конфигураций
// Преобразование в Kubernetes ресурсы
// Загрузка в реляционную модель
// Верификация целостности через SQLite FK
// Выявление и классификация ошибок
// Генерация планов исправления
package integrity

import (
	"fmt"
	"os"
	"strings"

	istio "istio.io/client-go/pkg/apis/networking/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/yaml"
)

// parseYAMLResources парсит YAML файл с множеством ресурсов
func parseYAMLResources(filePath string) (*RelationalModel, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	model := &RelationalModel{
		Services:         []ServiceRecord{},
		VirtualServices:  []VirtualServiceRecord{},
		Gateways:         []GatewayRecord{},
		DestinationRules: []DestinationRuleRecord{},
	}

	// Разделяем YAML документы
	documents := strings.Split(string(data), "---")

	for _, doc := range documents {
		doc = strings.TrimSpace(doc)
		if doc == "" {
			continue
		}

		// Определяем тип ресурса
		var typeMeta metav1.TypeMeta
		if err := yaml.Unmarshal([]byte(doc), &typeMeta); err != nil {
			continue
		}

		switch typeMeta.Kind {
		case "Service":
			var service corev1.Service
			if err := yaml.Unmarshal([]byte(doc), &service); err == nil {
				model.Services = append(model.Services, serviceToRecord(&service))
			}
		case "VirtualService":
			var vs istio.VirtualService
			if err := yaml.Unmarshal([]byte(doc), &vs); err == nil {
				model.VirtualServices = append(model.VirtualServices, virtualServiceToRecord(&vs))
			}
		case "DestinationRule":
			var dr istio.DestinationRule
			if err := yaml.Unmarshal([]byte(doc), &dr); err == nil {
				model.DestinationRules = append(model.DestinationRules, destinationRuleToRecord(&dr))
			}
		}
	}

	return model, nil
}

// serviceToRecord преобразует Kubernetes Service в ServiceRecord
func serviceToRecord(service *corev1.Service) ServiceRecord {
	if service == nil {
		return ServiceRecord{}
	}

	// Генерируем стандартный Kubernetes FQDN
	host := fmt.Sprintf("%s.%s.svc.cluster.local", service.Name, service.Namespace)

	record := ServiceRecord{
		Namespace: service.Namespace,
		Name:      service.Name,
		Host:      host,
		Protocol:  "TCP",
	}

	if len(service.Spec.Ports) > 0 {
		record.Port = service.Spec.Ports[0].Port
	}

	return record
}

// virtualServiceToRecord преобразует Istio VirtualService в VirtualServiceRecord
func virtualServiceToRecord(vs *istio.VirtualService) VirtualServiceRecord {
	if vs == nil {
		return VirtualServiceRecord{}
	}

	record := VirtualServiceRecord{
		Namespace: vs.Namespace,
		Name:      vs.Name,
		Host:      "", // Берем первый host из спецификации
	}

	// Получаем hosts
	if len(vs.Spec.Hosts) > 0 {
		record.Host = vs.Spec.Hosts[0]
	}

	// Получаем gateway reference
	if len(vs.Spec.Gateways) > 0 {
		gatewayParts := strings.Split(vs.Spec.Gateways[0], "/")
		if len(gatewayParts) == 2 {
			record.GatewayNamespace = gatewayParts[0]
			record.GatewayName = gatewayParts[1]
		} else if len(gatewayParts) == 1 {
			record.GatewayNamespace = vs.Namespace // gateway в том же namespace
			record.GatewayName = gatewayParts[0]
		}
	}

	// Получаем service reference из destination
	if len(vs.Spec.Http) > 0 && len(vs.Spec.Http[0].Route) > 0 {
		destination := vs.Spec.Http[0].Route[0].Destination
		if destination != nil {
			hostParts := strings.Split(destination.Host, ".")
			if len(hostParts) >= 2 {
				record.ServiceName = hostParts[0]
				record.ServiceNamespace = hostParts[1]
			}
		}
	}

	return record
}

// destinationRuleToRecord преобразует Istio DestinationRule в DestinationRuleRecord
func destinationRuleToRecord(dr *istio.DestinationRule) DestinationRuleRecord {
	if dr == nil {
		return DestinationRuleRecord{}
	}

	record := DestinationRuleRecord{
		Namespace:        dr.Namespace,
		Name:             dr.Name,
		ServiceNamespace: "",
		ServiceName:      "",
		Host:             dr.Spec.Host,
		TrafficPolicy:    dr.Spec.TrafficPolicy.String(),
	}

	// Преобразуем subsets в строку
	if len(dr.Spec.Subsets) > 0 {
		var subsetNames []string
		for _, subset := range dr.Spec.Subsets {
			subsetNames = append(subsetNames, subset.Name)
		}
		record.Subsets = strings.Join(subsetNames, ",")
	}

	// Получаем service reference из host
	hostParts := strings.Split(dr.Spec.Host, ".")
	if len(hostParts) >= 2 {
		record.ServiceNamespace = hostParts[1]
		record.ServiceName = hostParts[0]
	}

	return record
}
