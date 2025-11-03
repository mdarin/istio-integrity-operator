// Тесты для Repair Plans
package integrity

import (
	"database/sql"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	meshv1alpha1 "github.com/mdarin/istio-integrity-operator/api/v1alpha1"
)

func TestComputeRepairPlans(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	operator := &SQLiteIntegrityOperator{}

	// Create test report with violations
	report := &IntegrityReport{
		IsConsistent: false,
		Violations: []meshv1alpha1.ConstraintViolation{
			{
				Type:     "ForeignKeyViolation",
				Resource: "VirtualService/default/broken-vs",
				Message:  "References non-existent Gateway/istio-system/missing-gateway",
				Severity: "Error",
			},
			{
				Type:     "UniqueConstraintViolation",
				Resource: "Service/* (host: duplicate.svc.cluster.local, port: 8080)",
				Message:  "Duplicate host:port combination: duplicate.svc.cluster.local:8080 (2 services)",
				Severity: "Error",
			},
		},
	}

	// Compute repair plans
	repairs, err := operator.ComputeRepairPlans(db, report)
	if err != nil {
		t.Fatalf("Failed to compute repair plans: %v", err)
	}

	// Should generate repair plans for each violation
	if len(repairs) != 2 {
		t.Errorf("Expected 2 repair plans, got %d", len(repairs))
	}

	// Check specific repair actions
	foundDelete := false
	foundUpdate := false
	for _, repair := range repairs {
		t.Logf("🔧 Repair action: %v", repair)

		if repair.Type == "Delete" && repair.Resource == "VirtualService/default/broken-vs" {
			foundDelete = true
		}
		if repair.Type == "Update" && repair.Resource == "Service/* (host: duplicate.svc.cluster.local, port: 8080)" {
			foundUpdate = true
		}
	}

	if !foundDelete {
		t.Error("Expected delete repair plan for broken virtual service")
	}
	if !foundUpdate {
		t.Error("Expected update repair plan for duplicate host:port")
	}
}

func TestComputeRepairPlans_NoViolations(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	operator := &SQLiteIntegrityOperator{}

	// Create consistent report
	report := &IntegrityReport{
		IsConsistent: true,
		Violations:   []meshv1alpha1.ConstraintViolation{},
	}

	// Compute repair plans
	repairs, err := operator.ComputeRepairPlans(db, report)
	if err != nil {
		t.Fatalf("Failed to compute repair plans: %v", err)
	}

	// Should generate no repair plans for consistent model
	if len(repairs) != 0 {
		t.Errorf("Expected 0 repair plans for consistent model, got %d", len(repairs))
	}
}
