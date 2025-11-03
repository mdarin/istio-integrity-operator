// Интеграционный тест
package integrity

import (
	"testing"
)

// TestFullIntegrityWorkflow тестирует полный цикл работы системы integrity
func TestFullIntegrityWorkflow(t *testing.T) {
	operator := &SQLiteIntegrityOperator{}

	// 1. Создаем тестовую модель
	model := &RelationalModel{
		Services: []ServiceRecord{
			{Namespace: "default", Name: "frontend", Host: "frontend.default.svc.cluster.local", Port: 80, Protocol: "TCP"},
			{Namespace: "default", Name: "backend", Host: "backend.default.svc.cluster.local", Port: 3000, Protocol: "TCP"},
			{Namespace: "default", Name: "duplicate", Host: "frontend.default.svc.cluster.local", Port: 80, Protocol: "TCP"}, // Дубликат!
		},
		Gateways: []GatewayRecord{
			{Namespace: "istio-system", Name: "main-gateway"},
		},
		VirtualServices: []VirtualServiceRecord{
			{
				Namespace:        "default",
				Name:             "frontend-vs",
				GatewayNamespace: "istio-system",
				GatewayName:      "main-gateway",
				Host:             "app.example.com",
				ServiceNamespace: "default",
				ServiceName:      "frontend",
			},
			{
				Namespace:        "default",
				Name:             "backend-vs",
				GatewayNamespace: "istio-system",
				GatewayName:      "missing-gateway", // Несуществующий гейтвей!
				Host:             "api.example.com",
				ServiceNamespace: "default",
				ServiceName:      "backend",
			},
		},
	}

	// 2. Создаем in-memory БД
	db, err := operator.CreateInMemoryDB(model)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// 3. Проверяем целостность
	report, err := operator.CheckIntegrity(db)
	if err != nil {
		t.Fatalf("Failed to check integrity: %v", err)
	}

	// 4. Проверяем, что модель неконсистентна (как и ожидается)
	if !report.IsConsistent {
		t.Log("Expected model to be inconsistent")
		for _, violation := range report.Violations {
			t.Logf("⚠️ Violation: %s - %s", violation.Type, violation.Message)
		}
	}

	// 5. Проверяем количество нарушений (должно быть 2: дубликат и битая ссылка)
	expectedViolations := 3
	if len(report.Violations) != expectedViolations {
		t.Errorf("Expected %d violations, got %d", expectedViolations, len(report.Violations))
	}

	var serviceCount, vsCount, drCount, gwCount int
	db.QueryRow("SELECT COUNT(*) FROM services").Scan(&serviceCount)
	db.QueryRow("SELECT COUNT(*) FROM virtual_services").Scan(&vsCount)
	db.QueryRow("SELECT COUNT(*) FROM destination_rules").Scan(&drCount)
	db.QueryRow("SELECT COUNT(*) FROM gateways").Scan(&gwCount)

	t.Logf("✅ Database contains: %d gateways,  %d services, %d virtual services, %d destination rules",
		gwCount, serviceCount, vsCount, drCount)

	// 6. Вычисляем планы исправления
	repairs, err := operator.ComputeRepairPlans(db, report)
	if err != nil {
		t.Fatalf("Failed to compute repair plans: %v", err)
	}

	// 7. Проверяем, что созданы правильные планы исправления
	if len(repairs) != expectedViolations {
		t.Errorf("Expected %d repair plans, got %d", expectedViolations, len(repairs))
	}

	t.Logf("Integration test completed: %d violations, %d repair plans",
		len(report.Violations), len(repairs))
	for i, violation := range report.Violations {
		t.Logf("Violation %d: %s - %s", i+1, violation.Type, violation.Message)
	}
	for i, repair := range repairs {
		t.Logf("Repair %d: %s - %s", i+1, repair.Type, repair.Action)
	}
}
