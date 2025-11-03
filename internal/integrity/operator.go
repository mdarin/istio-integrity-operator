package integrity

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/mattn/go-sqlite3"
	meshv1alpha1 "github.com/mdarin/istio-integrity-operator/api/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

type SQLiteIntegrityOperator struct {
	client client.Client
}

func NewSQLiteIntegrityOperator(client client.Client) *SQLiteIntegrityOperator {
	return &SQLiteIntegrityOperator{
		client: client,
	}
}

// RelationalModel represents the in-memory relational model
type RelationalModel struct {
	Services         []ServiceRecord
	VirtualServices  []VirtualServiceRecord
	Gateways         []GatewayRecord
	DestinationRules []DestinationRuleRecord
}

type ServiceRecord struct {
	Namespace string
	Name      string
	Host      string
	Port      int32
	Protocol  string
}

type VirtualServiceRecord struct {
	Namespace        string
	Name             string
	GatewayNamespace string
	GatewayName      string
	Host             string
	ServiceNamespace string
	ServiceName      string
}

type GatewayRecord struct {
	Namespace string
	Name      string
}

// В Istio DestinationRule ссылается на Kubernetes Service, а не на VirtualService.
// ┌─────────────────┐    routes to    ┌──────────────────┐
// │ VirtualService  │ ──────────────> │  DestinationRule │
// │                 │                 │                  │
// └─────────────────┘                 └─────────┬────────┘
//                                               │
//                                       applies to
//                                               ▼
// ┌─────────────────────────────────────────────────┐
// │              Kubernetes Service                 │
// │                                                 │
// └─────────────────────────────────────────────────┘

type DestinationRuleRecord struct {
	Namespace        string
	Name             string
	ServiceNamespace string // ссылается на Kubernetes Service
	ServiceName      string // ссылается на Kubernetes Service
	Subsets          string
	TrafficPolicy    string
	// todo это надо валидирвоать по возможности!
	// host: reviews.prod.svc.cluster.local
	// Это полное DNS-имя Kubernetes-сервиса reviews в неймспейсе prod.
	// Istio использует это имя для сопоставления правил с трафиком, направляемым к этому сервису.
	// Указание полного доменного имени (<service>.<namespace>.svc.cluster.local) — рекомендуемая практика.
	Host string // host: reviews.prod.svc.cluster.local  # ← Kubernetes Service

}

// IntegrityReport contains the results of consistency checks
type IntegrityReport struct {
	IsConsistent bool
	Violations   []meshv1alpha1.ConstraintViolation
	RepairPlans  []meshv1alpha1.RepairAction
}

// BuildRelationalModel collects and transforms Kubernetes resources
func (o *SQLiteIntegrityOperator) BuildRelationalModel(ctx context.Context) (*RelationalModel, error) {
	log := log.FromContext(ctx)
	model := &RelationalModel{}

	// Collect all services
	var services corev1.ServiceList
	if err := o.client.List(ctx, &services); err != nil {
		return nil, fmt.Errorf("failed to list services: %w", err)
	}

	for _, svc := range services.Items {
		// Only process services with specific annotations or labels
		if o.shouldProcessService(&svc) {
			for _, port := range svc.Spec.Ports {
				model.Services = append(model.Services, ServiceRecord{
					Namespace: svc.Namespace,
					Name:      svc.Name,
					Host:      fmt.Sprintf("%s.%s.svc.cluster.local", svc.Name, svc.Namespace),
					Port:      port.Port,
					Protocol:  string(port.Protocol),
				})
			}
		}
	}

	log.Info("Built relational model", "services", len(model.Services))
	return model, nil
}

func (o *SQLiteIntegrityOperator) shouldProcessService(svc *corev1.Service) bool {
	// Add your logic to determine which services to process
	// For example, check for specific annotations
	_, hasMeshAnnotation := svc.Annotations["mesh.operator.istio.io/managed"]
	return hasMeshAnnotation
}

// CreateInMemoryDB creates SQLite in-memory database with schema
func (o *SQLiteIntegrityOperator) CreateInMemoryDB(model *RelationalModel) (*sql.DB, error) {
	db, err := sql.Open("sqlite3", "file::memory:?cache=shared")
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Включаем foreign keys
	if _, err := db.Exec("PRAGMA foreign_keys = ON;"); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	// Создаем схему
	if err := o.createSchema(db); err != nil {
		db.Close()
		return nil, err
	}

	// Загружаем данные
	if err := o.loadData(db, model); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to load data: %w", err)
	}

	return db, nil
}

func (o *SQLiteIntegrityOperator) createSchema(db *sql.DB) error {
	schema := `
    CREATE TABLE IF NOT EXISTS services (
        namespace TEXT NOT NULL,
        name TEXT NOT NULL,
        host TEXT NOT NULL, 
        port INTEGER NOT NULL,
        protocol TEXT NOT NULL,
        PRIMARY KEY (namespace, name)
    );

    CREATE TABLE IF NOT EXISTS gateways (
        namespace TEXT NOT NULL,
        name TEXT NOT NULL,
        PRIMARY KEY (namespace, name)
    );

    CREATE TABLE IF NOT EXISTS virtual_services (
        namespace TEXT NOT NULL,
        name TEXT NOT NULL,
        gateway_namespace TEXT NOT NULL,
        gateway_name TEXT NOT NULL,
        host TEXT NOT NULL,
        service_namespace TEXT NOT NULL,
        service_name TEXT NOT NULL,
        PRIMARY KEY (namespace, name),
        FOREIGN KEY (gateway_namespace, gateway_name) 
            REFERENCES gateways(namespace, name) ON DELETE CASCADE,
        FOREIGN KEY (service_namespace, service_name) 
            REFERENCES services(namespace, name) ON DELETE CASCADE
    );

    CREATE TABLE IF NOT EXISTS destination_rules (
        namespace TEXT NOT NULL,
        name TEXT NOT NULL,
        service_namespace TEXT NOT NULL,  -- ссылается на k8s service
        service_name TEXT NOT NULL,       -- ссылается на k8s service  
        subsets TEXT,
        traffic_policy TEXT,
		host TEXT NOT NULL,               -- ссылается на k8s service 
        PRIMARY KEY (namespace, name),
        FOREIGN KEY (service_namespace, service_name) 
            REFERENCES services(namespace, name) ON DELETE CASCADE
    );

    CREATE INDEX IF NOT EXISTS idx_vs_host_gateway 
        ON virtual_services(host, gateway_namespace, gateway_name);
	CREATE INDEX IF NOT EXISTS idx_services_host_port 
		ON services(host, port);
	CREATE INDEX IF NOT EXISTS idx_services_ns_name 
		ON services(namespace, name);
	CREATE INDEX IF NOT EXISTS idx_vs_svc_ref 
		ON virtual_services(service_namespace, service_name);
	CREATE INDEX IF NOT EXISTS idx_vs_host_gw 
		ON virtual_services(host, gateway_namespace, gateway_name);
    `

	_, err := db.Exec(schema)
	return err
}

func (o *SQLiteIntegrityOperator) loadData(db *sql.DB, model *RelationalModel) error {
	if _, err := db.Exec("PRAGMA foreign_keys = OFF"); err != nil {
		return fmt.Errorf("failed to disable foreign keys: %w", err)
	}

	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Загружаем данные и собираем ВСЕ нарушения
	for _, gw := range model.Gateways {
		if _, err := tx.Exec(
			"INSERT INTO gateways (namespace, name) VALUES (?, ?)",
			gw.Namespace, gw.Name,
		); err != nil {
			return err
		}
	}

	for _, svc := range model.Services {
		if _, err := tx.Exec(
			"INSERT INTO services (namespace, name, host, port, protocol) VALUES (?, ?, ?, ?, ?)",
			svc.Namespace, svc.Name, svc.Host, svc.Port, svc.Protocol,
		); err != nil {
			return err
		}
	}

	for _, vs := range model.VirtualServices {
		if _, err := tx.Exec(
			"INSERT INTO virtual_services (namespace, name, gateway_namespace, gateway_name, host, service_namespace, service_name) VALUES (?, ?, ?, ?, ?, ?, ?)",
			vs.Namespace, vs.Name, vs.GatewayNamespace, vs.GatewayName, vs.Host, vs.ServiceNamespace, vs.ServiceName,
		); err != nil {
			return err
		}
	}

	for _, dr := range model.DestinationRules {
		if _, err := tx.Exec(
			"INSERT INTO destination_rules (namespace, name, host, subsets, service_namespace, service_name) VALUES (?, ?, ?, ?, ?, ?)",
			dr.Namespace, dr.Name, dr.Host, dr.Subsets, dr.ServiceNamespace, dr.ServiceName,
		); err != nil {
			return err
		}
	}

	// Коммитим транзакцию даже с нарушениями - они уже зафиксированы в отчете
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Включаем FK обратно
	if _, err := db.Exec("PRAGMA foreign_keys = ON"); err != nil {
		return fmt.Errorf("failed to enable foreign keys: %w", err)
	}

	return nil
}

// CheckIntegrity performs comprehensive referential and uniqueness integrity checks
func (o *SQLiteIntegrityOperator) CheckIntegrity(db *sql.DB) (*IntegrityReport, error) {
	report := &IntegrityReport{IsConsistent: true}

	// 1. Check foreign key violations
	fkViolations, err := o.checkForeignKeyViolations(db)
	if err != nil {
		return nil, fmt.Errorf("failed to check foreign key violations: %w", err)
	}
	report.Violations = append(report.Violations, fkViolations...)

	// 2. Check unique constraint violations
	uniqueViolations, err := o.checkUniqueConstraintViolations(db)
	if err != nil {
		return nil, fmt.Errorf("failed to check unique constraint violations: %w", err)
	}
	report.Violations = append(report.Violations, uniqueViolations...)

	// Final consistency flag
	report.IsConsistent = len(report.Violations) == 0
	return report, nil
}

// checkForeignKeyViolations проверяет все логические ссылки
func (o *SQLiteIntegrityOperator) checkForeignKeyViolations(db *sql.DB) ([]meshv1alpha1.ConstraintViolation, error) {
	var violations []meshv1alpha1.ConstraintViolation

	// 1. VirtualService -> Gateway
	rows, err := db.Query(`
		SELECT vs.namespace, vs.name, vs.gateway_namespace, vs.gateway_name
		FROM virtual_services vs
		LEFT JOIN gateways gw ON vs.gateway_namespace = gw.namespace AND vs.gateway_name = gw.name
		WHERE gw.namespace IS NULL
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ns, name, gwNs, gwName string
		if err := rows.Scan(&ns, &name, &gwNs, &gwName); err != nil {
			rows.Close()
			return nil, err
		}
		violations = append(violations, meshv1alpha1.ConstraintViolation{
			Type:     "ForeignKeyViolation",
			Resource: fmt.Sprintf("VirtualService/%s/%s", ns, name),
			Message:  fmt.Sprintf("References non-existent Gateway/%s/%s", gwNs, gwName),
			Severity: "Error",
		})
	}
	rows.Close()

	// 2. VirtualService -> Service
	rows, err = db.Query(`
		SELECT vs.namespace, vs.name, vs.service_namespace, vs.service_name
		FROM virtual_services vs
		LEFT JOIN services s ON vs.service_namespace = s.namespace AND vs.service_name = s.name
		WHERE s.namespace IS NULL
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ns, name, svcNs, svcName string
		if err := rows.Scan(&ns, &name, &svcNs, &svcName); err != nil {
			rows.Close()
			return nil, err
		}
		violations = append(violations, meshv1alpha1.ConstraintViolation{
			Type:     "ForeignKeyViolation",
			Resource: fmt.Sprintf("VirtualService/%s/%s", ns, name),
			Message:  fmt.Sprintf("References non-existent Service/%s/%s", svcNs, svcName),
			Severity: "Error",
		})
	}
	rows.Close()

	// 3. DestinationRule -> Service (by namespace/name)
	rows, err = db.Query(`
		SELECT dr.namespace, dr.name, dr.service_namespace, dr.service_name
		FROM destination_rules dr
		LEFT JOIN services s ON dr.service_namespace = s.namespace AND dr.service_name = s.name
		WHERE s.namespace IS NULL
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var ns, name, svcNs, svcName string
		if err := rows.Scan(&ns, &name, &svcNs, &svcName); err != nil {
			rows.Close()
			return nil, err
		}
		violations = append(violations, meshv1alpha1.ConstraintViolation{
			Type:     "ForeignKeyViolation",
			Resource: fmt.Sprintf("DestinationRule/%s/%s", ns, name),
			Message:  fmt.Sprintf("References non-existent Service/%s/%s", svcNs, svcName),
			Severity: "Error",
		})
	}
	rows.Close()

	// 4. (Опционально) DestinationRule -> Service by host?
	// ❌ Рекомендуется УДАЛИТЬ из схемы FOREIGN KEY (host) REFERENCES services(host)
	// Потому что host — не ключ, и может быть несколько сервисов с одним host? (в норме — нет)
	// Но если вы всё же хотите проверить:
	/*
		rows, err = db.Query(`
			SELECT dr.namespace, dr.name, dr.host
			FROM destination_rules dr
			LEFT JOIN services s ON dr.host = s.host
			WHERE s.host IS NULL
		`)
		... аналогично ...
	*/

	return violations, nil
}

// checkUniqueConstraintViolations проверяет все важные уникальности
func (o *SQLiteIntegrityOperator) checkUniqueConstraintViolations(db *sql.DB) ([]meshv1alpha1.ConstraintViolation, error) {
	var violations []meshv1alpha1.ConstraintViolation

	// 1. Дубликаты host:port в services
	rows, err := db.Query(`
		SELECT host, port, COUNT(*) as count
		FROM services
		GROUP BY host, port
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var host string
		var port int32
		var count int
		if err := rows.Scan(&host, &port, &count); err != nil {
			rows.Close()
			return nil, err
		}
		violations = append(violations, meshv1alpha1.ConstraintViolation{
			Type:     "UniqueConstraintViolation",
			Resource: fmt.Sprintf("Service/* (host: %s, port: %d)", host, port),
			Message:  fmt.Sprintf("Duplicate host:port combination: %s:%d (%d services)", host, port, count),
			Severity: "Error",
		})
	}
	rows.Close()

	// 2. Дубликаты host в services (если важно)
	rows, err = db.Query(`
		SELECT host, COUNT(*) as count
		FROM services
		GROUP BY host
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var host string
		var count int
		if err := rows.Scan(&host, &count); err != nil {
			rows.Close()
			return nil, err
		}
		// Проверим, не дублируется ли уже по host:port — чтобы не дублировать сообщение
		// Но если вы НЕ проверяете host:port, или если порты разные — это отдельная ошибка
		violations = append(violations, meshv1alpha1.ConstraintViolation{
			Type:     "UniqueConstraintViolation",
			Resource: fmt.Sprintf("Service/* (host: %s)", host),
			Message:  fmt.Sprintf("Multiple services share the same host: %s (%d services)", host, count),
			Severity: "Warning", // или "Error", в зависимости от политики
		})
	}
	rows.Close()

	// 3. Дубликаты (host, gateway) в virtual_services
	// В Istio: один host на gateway должен обслуживаться одним VirtualService
	rows, err = db.Query(`
		SELECT host, gateway_namespace, gateway_name, COUNT(*) as count
		FROM virtual_services
		GROUP BY host, gateway_namespace, gateway_name
		HAVING COUNT(*) > 1
	`)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var host, gwNs, gwName string
		var count int
		if err := rows.Scan(&host, &gwNs, &gwName, &count); err != nil {
			rows.Close()
			return nil, err
		}
		violations = append(violations, meshv1alpha1.ConstraintViolation{
			Type:     "UniqueConstraintViolation",
			Resource: fmt.Sprintf("VirtualService/* (host: %s, gateway: %s/%s)", host, gwNs, gwName),
			Message:  fmt.Sprintf("Multiple VirtualServices define the same host %s for Gateway %s/%s (%d vs)", host, gwNs, gwName, count),
			Severity: "Error",
		})
	}
	rows.Close()

	return violations, nil
}

// ComputeRepairPlans generates repair actions based on violations
func (o *SQLiteIntegrityOperator) ComputeRepairPlans(db *sql.DB, report *IntegrityReport) ([]meshv1alpha1.RepairAction, error) {
	var repairs []meshv1alpha1.RepairAction

	for _, violation := range report.Violations {
		switch violation.Type {
		case "ForeignKeyViolation":
			// For broken VirtualService references, plan to delete the VirtualService
			if strings.HasPrefix(violation.Resource, "VirtualService/") {
				parts := strings.Split(violation.Resource, "/")
				if len(parts) == 3 {
					repairs = append(repairs, meshv1alpha1.RepairAction{
						Type:     "Delete",
						Resource: violation.Resource,
						Action:   "Delete broken VirtualService reference",
						Reason:   violation.Message,
					})
				}
			}
		case "UniqueConstraintViolation":
			repairs = append(repairs, meshv1alpha1.RepairAction{
				Type:     "Update",
				Resource: violation.Resource,
				Action:   "Resolve host:port conflict",
				Reason:   violation.Message,
			})
		}
	}

	return repairs, nil
}

// GetMeshService retrieves a specific MeshService CR
func (o *SQLiteIntegrityOperator) GetMeshService(ctx context.Context, namespacedName types.NamespacedName) (*meshv1alpha1.MeshService, error) {
	var meshService meshv1alpha1.MeshService
	if err := o.client.Get(ctx, namespacedName, &meshService); err != nil {
		return nil, err
	}
	return &meshService, nil
}
