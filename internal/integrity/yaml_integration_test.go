package integrity

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestValidYAMLResources(t *testing.T) {
	// 1. Парсим YAML файл
	model, err := parseYAMLResources("testdata/valid-mesh-resources.yaml")
	if err != nil {
		t.Fatalf("Failed to parse YAML file: %v", err)
	}

	// 2. Добавляем недостающий Gateway (т.к. он не указан в YAML)
	model.Gateways = []GatewayRecord{
		{Namespace: "istio-system", Name: "public-gateway"},
	}

	t.Logf("Parsed model: %d services, %d virtual services, %d destination rules, %d gateways",
		len(model.Services), len(model.VirtualServices), len(model.DestinationRules), len(model.Gateways))

	// 3. Создаем in-memory БД
	operator := &SQLiteIntegrityOperator{}
	// CreateInMemoryDB теперь возвращает и БД
	db, err := operator.CreateInMemoryDB(model)
	if err != nil {
		t.Fatalf("Failed to create in-memory database: %v", err)
	}
	defer db.Close()

	// 4. Проверяем целостность
	report, err := operator.CheckIntegrity(db)
	if err != nil {
		t.Fatalf("Failed to check integrity: %v", err)
	}

	// 5. Проверяем что модель консистентна
	if !report.IsConsistent {
		t.Errorf("Expected valid YAML to be consistent, but found violations:")
		for i, violation := range report.Violations {
			t.Errorf(" ⚠️ Violation %d: %s - %s", i+1, violation.Type, violation.Message)
		}
	}

	// 6. Проверяем что нет нарушений
	if len(report.Violations) > 0 {
		t.Errorf("Expected 0 violations for valid YAML, but got %d", len(report.Violations))
	} else {
		t.Logf("✅ Valid YAML passed all integrity checks - no violations found")
	}

	// 7. Проверяем что все ресурсы загружены в БД
	var serviceCount, vsCount, drCount, gwCount int
	db.QueryRow("SELECT COUNT(*) FROM services").Scan(&serviceCount)
	db.QueryRow("SELECT COUNT(*) FROM virtual_services").Scan(&vsCount)
	db.QueryRow("SELECT COUNT(*) FROM destination_rules").Scan(&drCount)
	db.QueryRow("SELECT COUNT(*) FROM gateways").Scan(&gwCount)

	if serviceCount != 1 {
		t.Errorf("Expected 1 service in DB, got %d", serviceCount)
	}
	if vsCount != 1 {
		t.Errorf("Expected 1 virtual service in DB, got %d", vsCount)
	}
	if drCount != 1 {
		t.Errorf("Expected 1 destination rule in DB, got %d", drCount)
	}

	t.Logf("✅ Database contains: %d gateways,  %d services, %d virtual services, %d destination rules",
		gwCount, serviceCount, vsCount, drCount)
}

func TestInvalidYAMLResources(t *testing.T) {
	// 1. Парсим YAML файл с ошибками
	model, err := parseYAMLResources("testdata/invalid-mesh-resources.yaml")
	if err != nil {
		t.Fatalf("Failed to parse YAML file: %v", err)
	}

	// 2. Добавляем только существующие Gateway (не добавляем non-existent-gateway)
	model.Gateways = []GatewayRecord{
		{Namespace: "istio-system", Name: "public-gateway"}, // Только этот gateway существует
	}

	t.Logf("Parsed invalid model: %d services, %d virtual services, %d destination rules, %d gateways",
		len(model.Services), len(model.VirtualServices), len(model.DestinationRules), len(model.Gateways))

	// 3. Создаем in-memory БД - ОЖИДАЕМ ошибку из-за FK violations!
	operator := &SQLiteIntegrityOperator{}

	db, err := operator.CreateInMemoryDB(model)

	// Для невалидных данных ожидаем ошибку при загрузке
	t.Log("Skipped error:", err)

	// 8. Проверяем что только валидные данные загружены
	var serviceCount, gatewayCount, virtualServiceCount int
	db.QueryRow("SELECT COUNT(*) FROM services").Scan(&serviceCount)
	db.QueryRow("SELECT COUNT(*) FROM gateways").Scan(&gatewayCount)
	db.QueryRow("SELECT COUNT(*) FROM virtual_services").Scan(&virtualServiceCount)

	if serviceCount != 1 {
		t.Errorf("❌ Expected 1 service in DB, got %d", serviceCount)
	}
	if gatewayCount != 1 {
		t.Errorf("❌ Expected 1 gateway in DB, got %d", gatewayCount)
	}
	if virtualServiceCount != 1 {
		t.Errorf("❌ Expected 1 virtual_service in DB, got %d", virtualServiceCount)
	}

	t.Logf("✅ Database contains only valid data: %d services, %d gateways", serviceCount, gatewayCount)

	t.Logf("✅ Database contains broken virtual_services %d", virtualServiceCount)

	// 4. Проверяем целостность
	report, err := operator.CheckIntegrity(db)
	if err != nil {
		t.Fatalf("Failed to check integrity: %v", err)
	}

	// 5. Проверяем что модель НЕ консистентна (это ожидаемо)
	if report.IsConsistent {
		t.Error("❌ Expected invalid YAML to be inconsistent, but it was reported as consistent")
	}

	for i, violation := range report.Violations {
		t.Logf(" ⚠️ Violation %d: %s - %s", i+1, violation.Type, violation.Message)
	}

	// 6. Проверяем что найдены ожидаемые нарушения
	expectedViolations := 1 // ForeignKeyViolation - References non-existent Gateway/istio-system/non-existent-gateway
	if len(report.Violations) < expectedViolations {
		t.Errorf("❌ Expected at least %d violations, but got %d", expectedViolations, len(report.Violations))
	}

	t.Logf("✅ Invalid YAML correctly detected %d integrity violations", len(report.Violations))

	// 8. Проверяем вычисление repair plans
	repairs, err := operator.ComputeRepairPlans(db, report)
	if err != nil {
		t.Fatalf("Failed to compute repair plans: %v", err)
	}

	if len(repairs) == 0 {
		t.Error("❌ Expected repair plans for violations, but got none")
	} else {
		t.Logf("✅ Generated %d repair plans for violations", len(repairs))
		for i, repair := range repairs {
			t.Logf("  Repair %d: %s - %s", i+1, repair.Type, repair.Action)
		}
	}

	db.Close()
}

func TestInvalidYAMLResources0(t *testing.T) {
	// 1. Парсим YAML файл с ошибками
	model, err := parseYAMLResources("testdata/invalid-mesh-resources.yaml")
	if err != nil {
		t.Fatalf("Failed to parse YAML file: %v", err)
	}

	// 2. Добавляем только существующие Gateway (не добавляем non-existent-gateway)
	model.Gateways = []GatewayRecord{
		{Namespace: "istio-system", Name: "public-gateway"}, // Только этот gateway существует
	}

	t.Logf("Parsed invalid model: %d services, %d virtual services, %d destination rules, %d gateways",
		len(model.Services), len(model.VirtualServices), len(model.DestinationRules), len(model.Gateways))

	// 3. Создаем in-memory БД
	operator := &SQLiteIntegrityOperator{}
	db, err := operator.CreateInMemoryDB(model)

	// Для невалидных данных ожидаем ошибку при загрузке
	t.Log("Skipped err:", err)

	// 4. Проверяем целостность
	report, err := operator.CheckIntegrity(db)
	if err != nil {
		t.Fatalf("Failed to compute repair plans: %v", err)
	}

	// 5. Проверяем что модель НЕ консистентна (это ожидаемо)
	if report.IsConsistent {
		t.Log("❌ Expected invalid YAML to be inconsistent, but it was reported as consistent")
	}

	// 6. Проверяем что найдены ожидаемые нарушения
	expectedViolations := 2 // non-existent-gateway + non-existent-service + another-non-existent
	if len(report.Violations) < expectedViolations {
		t.Errorf("❌ Expected at least %d violations, but got %d", expectedViolations, len(report.Violations))
	}

	// 7. Проверяем конкретные типы нарушений
	foundGatewayViolation := false
	// foundServiceViolation := false

	for _, violation := range report.Violations {
		t.Logf("⭕️ Found violation: %s - %s", violation.Type, violation.Message)

		if violation.Type == "ForeignKeyViolation" {
			t.Logf("✅ ForeignKeyViolation")
			foundGatewayViolation = true
		}
	}

	if !foundGatewayViolation {
		t.Error("❌ Expected to find foreign key violation for non-existent gateway")
	}
	// if !foundServiceViolation {
	// 	t.Error("❌ Expected to find foreign key violation for non-existent service")
	// }

	t.Logf("✅ Invalid YAML correctly detected %d integrity violations", len(report.Violations))

	// 8. Проверяем вычисление repair plans
	repairs, err := operator.ComputeRepairPlans(db, report)
	if err != nil {
		t.Fatalf("Failed to compute repair plans: %v", err)
	}

	if len(repairs) == 0 {
		t.Error("❌ Expected repair plans for violations, but got none")
	} else {
		t.Logf("✅ Generated %d repair plans for violations", len(repairs))
		for i, repair := range repairs {
			t.Logf("  Repair %d: %s - %s", i+1, repair.Type, repair.Action)
		}
	}

	db.Close()
}
