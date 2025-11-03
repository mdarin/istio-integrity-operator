// Тесты для проверки целостности
package integrity

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/mdarin/istio-integrity-operator/pkg/sqliteerrors"
)

func TestCheckForeignKeyViolations(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	operator := &SQLiteIntegrityOperator{}

	// Create schema
	if err := operator.createSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Setup test data with intentional FK violation
	testData := `
		-- Insert services
		INSERT INTO services (namespace, name, host, port, protocol) VALUES
		('default', 'web', 'web.default.svc.cluster.local', 8080, 'TCP'),
		('default', 'api', 'api.default.svc.cluster.local', 9090, 'TCP');
		
		-- Insert gateways
		INSERT INTO gateways (namespace, name) VALUES
		('istio-system', 'public-gateway');
		
		-- Insert valid virtual service
		INSERT INTO virtual_services (namespace, name, gateway_namespace, gateway_name, host, service_namespace, service_name) VALUES
		('default', 'valid-vs', 'istio-system', 'public-gateway', 'web.example.com', 'default', 'web');
		
		-- Insert broken virtual service (non-existent gateway)
		INSERT INTO virtual_services (namespace, name, gateway_namespace, gateway_name, host, service_namespace, service_name) VALUES
		('default', 'broken-vs', 'istio-system', 'non-existent-gateway', 'broken.example.com', 'default', 'api');
	`

	// Temporarily disable foreign keys to insert test data
	_, err = db.Exec("PRAGMA foreign_keys = OFF")
	if err != nil {
		t.Fatalf("Failed to disable foreign keys: %v", err)
	}

	_, err = db.Exec(testData)
	if err != nil {
		t.Fatalf("Failed to insert test data: %v", err)
	}

	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Check for violations
	violations, err := operator.checkForeignKeyViolations(db)
	if err != nil {
		t.Fatalf("Failed to check foreign key violations: %v", err)
	}

	// Should find exactly 1 violation
	if len(violations) != 1 {
		t.Errorf("Expected 1 foreign key violation, got %d", len(violations))
	}

	if len(violations) > 0 {
		violation := violations[0]
		if violation.Type != "ForeignKeyViolation" {
			t.Errorf("Expected violation type 'ForeignKeyViolation', got '%s'", violation.Type)
		}
		if violation.Resource != "VirtualService/default/broken-vs" {
			t.Errorf("Unexpected violation resource: %s", violation.Resource)
		}
	}
}

func TestCheckUniqueConstraintViolations(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	operator := &SQLiteIntegrityOperator{}

	// Create schema
	if err := operator.createSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Setup test data with duplicate host:port
	testData := `
		INSERT INTO services (namespace, name, host, port, protocol) VALUES
		('default', 'web1', 'same.host.svc.cluster.local', 8080, 'TCP'),
		('default', 'web2', 'same.host.svc.cluster.local', 8080, 'TCP'),  -- Duplicate host:port
		('default', 'api', 'unique.host.svc.cluster.local', 9090, 'TCP');
	`

	_, err = db.Exec(testData)
	if err == nil {
		t.Fatalf("Expected failed to insert test data UNIQUE constraint failed: services.host")
	}

	if sqliteerrors.E(err).Unique().Is() {
		t.Logf("Expected %s", err)
	}

	// // Check for unique constraint violations
	// violations, err := operator.checkUniqueConstraintViolations(db)
	// if err != nil {
	// 	t.Fatalf("Failed to check unique constraint violations: %v", err)
	// }

	// // Should find exactly 1 violation for duplicate host:port
	// if len(violations) != 1 {
	// 	t.Errorf("Expected 1 unique constraint violation, got %d", len(violations))
	// }

	// if len(violations) > 0 {
	// 	violation := violations[0]
	// 	if violation.Type != "UniqueConstraintViolation" {
	// 		t.Errorf("Expected violation type 'UniqueConstraintViolation', got '%s'", violation.Type)
	// 	}
	// 	if violation.Severity != "Error" {
	// 		t.Errorf("Expected severity 'Error', got '%s'", violation.Severity)
	// 	}
	// }
}

func TestCheckIntegrity_ConsistentModel(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	operator := &SQLiteIntegrityOperator{}

	// Create schema
	if err := operator.createSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Setup consistent test data
	model := &RelationalModel{
		Services: []ServiceRecord{
			{Namespace: "default", Name: "web", Host: "web.default.svc.cluster.local", Port: 8080, Protocol: "TCP"},
			{Namespace: "default", Name: "api", Host: "api.default.svc.cluster.local", Port: 9090, Protocol: "TCP"},
		},
		Gateways: []GatewayRecord{
			{Namespace: "istio-system", Name: "public-gateway"},
			{Namespace: "istio-system", Name: "internal-gateway"},
		},
		VirtualServices: []VirtualServiceRecord{
			{
				Namespace:        "default",
				Name:             "web-vs",
				GatewayNamespace: "istio-system",
				GatewayName:      "public-gateway",
				Host:             "web.example.com",
				ServiceNamespace: "default",
				ServiceName:      "web",
			},
			{
				Namespace:        "default",
				Name:             "api-vs",
				GatewayNamespace: "istio-system",
				GatewayName:      "internal-gateway",
				Host:             "api.internal.com",
				ServiceNamespace: "default",
				ServiceName:      "api",
			},
		},
	}

	// Load data
	if err := operator.loadData(db, model); err != nil {
		t.Fatalf("Failed to load data: %v", err)
	}

	// Check integrity
	report, err := operator.CheckIntegrity(db)
	if err != nil {
		t.Fatalf("Failed to check integrity: %v", err)
	}

	if !report.IsConsistent {
		t.Error("Expected model to be consistent, but found violations")
		for _, violation := range report.Violations {
			t.Errorf("⚠️ Violation: %s - %s", violation.Type, violation.Message)
		}
	}

	if len(report.Violations) != 0 {
		t.Errorf("Expected 0 violations, got %d", len(report.Violations))
	}
}

func TestCheckIntegrity_InconsistentModel(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	operator := &SQLiteIntegrityOperator{}

	// Create schema
	if err := operator.createSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Setup inconsistent test data with broken references
	model := &RelationalModel{
		Services: []ServiceRecord{
			{Namespace: "default", Name: "web", Host: "web.default.svc.cluster.local", Port: 8080, Protocol: "TCP"},
		},
		// Intentionally missing gateways
		Gateways: []GatewayRecord{},
		VirtualServices: []VirtualServiceRecord{
			{
				Namespace:        "default",
				Name:             "broken-vs",
				GatewayNamespace: "istio-system", // This gateway doesn't exist
				GatewayName:      "non-existent-gateway",
				Host:             "broken.example.com",
				ServiceNamespace: "default",
				ServiceName:      "web",
			},
		},
	}

	// Load data (should succeed since we're not enforcing FKs during load in test)
	if err := operator.loadData(db, model); err != nil {
		t.Fatalf("Failed to load data: %v", err)
	}

	// Check integrity
	report, err := operator.CheckIntegrity(db)
	if err != nil {
		t.Fatalf("Failed to check integrity: %v", err)
	}

	if report.IsConsistent {
		t.Error("Expected model to be inconsistent, but it was reported as consistent")
	}

	if len(report.Violations) == 0 {
		t.Error("Expected violations in inconsistent model, but found none")
	}

	t.Log("✅ Expected model to be consistent, but found violations")
	for _, violation := range report.Violations {
		t.Logf("⚠️ Violation: %s - %s", violation.Type, violation.Message)
	}
}
