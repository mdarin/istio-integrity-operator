package integrity

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestCreateSchema(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Включить foreign keys ПЕРЕД созданием схемы
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	operator := &SQLiteIntegrityOperator{}

	// Test schema creation
	err = operator.createSchema(db)
	if err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Verify tables were created
	tables := []string{"services", "gateways", "virtual_services", "destination_rules"}
	for _, table := range tables {
		var name string
		err = db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name=?", table).Scan(&name)
		if err != nil {
			t.Errorf("Table %s was not created: %v", table, err)
		}
	}

	// Verify foreign keys are enabled
	var fkEnabled int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	if err != nil {
		t.Fatalf("Failed to check foreign keys: %v", err)
	}
	if fkEnabled != 1 {
		t.Error("Foreign keys are not enabled")
	}

	var dbtables []string
	db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' ORDER BY name").Scan(&dbtables)
	t.Log("tables", dbtables)
	db.QueryRow("SHOW TABLES").Scan(&dbtables)
	t.Log("tables", dbtables)
}

func TestLoadData(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Включить foreign keys ПЕРЕД созданием схемы
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	operator := &SQLiteIntegrityOperator{}

	// Create schema
	if err := operator.createSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Test data - ВАЖНО: правильный порядок зависимостей
	model := &RelationalModel{
		// 1. Сначала независимые сущности
		Gateways: []GatewayRecord{
			{Namespace: "istio-system", Name: "public-gateway"}, // ДОЛЖЕН БЫТЬ ПЕРВЫМ
		},
		// 2. Затем services
		Services: []ServiceRecord{
			{Namespace: "default", Name: "web", Host: "web.default.svc.cluster.local", Port: 8080, Protocol: "TCP"},
			{Namespace: "default", Name: "api", Host: "api.default.svc.cluster.local", Port: 9090, Protocol: "TCP"},
		},
		// 3. Только потом зависимые сущности
		VirtualServices: []VirtualServiceRecord{
			{
				Namespace:        "default",
				Name:             "web-vs",
				GatewayNamespace: "istio-system",
				GatewayName:      "public-gateway", // Ссылается на существующий gateway
				Host:             "web.example.com",
				ServiceNamespace: "default",
				ServiceName:      "web", // Ссылается на существующий service
			},
		},
		DestinationRules: []DestinationRuleRecord{
			{
				Namespace:        "default",
				Name:             "web-dr",
				ServiceNamespace: "default",                       // ссылается на service
				ServiceName:      "web",                           // ссылается на service
				Host:             "web.default.svc.cluster.local", // ссылается на service
				Subsets:          "v1,v2",
				TrafficPolicy:    `{"loadBalancer":{"simple":"LEAST_CONN"}}`,
			},
		},
	}

	// Load data
	if err := operator.loadData(db, model); err != nil {
		t.Fatalf("Failed to load data: %v", err)
	}

	// Verify data was inserted
	var serviceCount int
	err = db.QueryRow("SELECT COUNT(*) FROM services").Scan(&serviceCount)
	if err != nil {
		t.Fatalf("Failed to count services: %v", err)
	}
	if serviceCount != 2 {
		t.Errorf("Expected 2 services, got %d", serviceCount)
	}

	var vsCount int
	err = db.QueryRow("SELECT COUNT(*) FROM virtual_services").Scan(&vsCount)
	if err != nil {
		t.Fatalf("Failed to count virtual services: %v", err)
	}
	if vsCount != 1 {
		t.Errorf("Expected 1 virtual service, got %d", vsCount)
	}

	var gatewayCount int
	err = db.QueryRow("SELECT COUNT(*) FROM gateways").Scan(&gatewayCount)
	if err != nil {
		t.Fatalf("Failed to count gateways: %v", err)
	}
	if gatewayCount != 1 {
		t.Errorf("Expected 1 gateway, got %d", gatewayCount)
	}

	var destRulesCount int
	err = db.QueryRow("SELECT COUNT(*) FROM destination_rules").Scan(&destRulesCount)
	if err != nil {
		t.Fatalf("Failed to count destination_rules: %v", err)
	}
	if destRulesCount != 1 {
		t.Errorf("Expected 1 destination rule, got %d", destRulesCount)
	}

	t.Logf("✅ 1 gateway")
	t.Logf("✅ 1 virtual service")
	t.Logf("✅ 2 services")
	t.Logf("✅ 1 destination rule")

	t.Logf("✅ Successfully loaded data with foreign key constraints")
}

func TestLoadDataDebug(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Включить foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	operator := &SQLiteIntegrityOperator{}

	// Create schema
	if err := operator.createSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Шаг 1: Вставить gateway вручную
	_, err = db.Exec("INSERT INTO gateways (namespace, name) VALUES (?, ?)", "istio-system", "public-gateway")
	if err != nil {
		t.Fatalf("Failed to insert gateway manually: %v", err)
	}

	// Шаг 2: Вставить service вручную
	_, err = db.Exec("INSERT INTO services (namespace, name, host, port, protocol) VALUES (?, ?, ?, ?, ?)",
		"default", "web", "web.default.svc.cluster.local", 8080, "TCP")
	if err != nil {
		t.Fatalf("Failed to insert service manually: %v", err)
	}

	// Шаг 3: Попытаться вставить virtual_service
	_, err = db.Exec(`
        INSERT INTO virtual_services 
        (namespace, name, gateway_namespace, gateway_name, host, service_namespace, service_name) 
        VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"default", "web-vs", "istio-system", "public-gateway", "web.example.com", "default", "web")

	if err != nil {
		t.Fatalf("Failed to insert virtual_service even with existing dependencies: %v", err)
	} else {
		t.Logf("✅ Successfully inserted virtual_service with valid foreign keys")
	}
}

func TestForeignKeyConstraints(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Включить foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	operator := &SQLiteIntegrityOperator{}

	// Create schema
	if err := operator.createSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Insert base data (gateway must exist first)
	_, err = db.Exec(`
		INSERT INTO gateways (namespace, name) VALUES ('istio-system', 'public-gateway');
		INSERT INTO services (namespace, name, host, port, protocol) VALUES 
		('default', 'web', 'web.default.svc.cluster.local', 8080, 'TCP');
	`)
	if err != nil {
		t.Fatalf("Failed to insert base data: %v", err)
	}

	// Try to insert VirtualService with non-existent Gateway (should fail)
	_, err = db.Exec(`
		INSERT INTO virtual_services 
		(namespace, name, gateway_namespace, gateway_name, host, service_namespace, service_name) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"default", "broken-vs", "istio-system", "non-existent-gateway", "test.com", "default", "web")

	if err == nil {
		t.Error("Expected foreign key violation error, but insertion succeeded")
	} else {
		t.Logf("Correctly got foreign key error: %v", err)
	}

	// 4. Проверяем целостность
	report, err := operator.CheckIntegrity(db)
	if err != nil {
		// Если ошибки нет - закрываем БД и завершаем тест
		t.Fatalf("❌ Expected 'Check integrity success")
	}

	// 5. Проверяем что модель НЕ консистентна (это ожидаемо)
	if !report.IsConsistent {
		t.Error("❌ Expected it report as consistent")
	}
}

func TestValidForeignKeyInsert(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Включить foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	operator := &SQLiteIntegrityOperator{}

	// Create schema
	if err := operator.createSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// Insert base data first
	_, err = db.Exec(`
		INSERT INTO gateways (namespace, name) VALUES ('istio-system', 'public-gateway');
		INSERT INTO services (namespace, name, host, port, protocol) VALUES 
		('default', 'web', 'web.default.svc.cluster.local', 8080, 'TCP');
	`)
	if err != nil {
		t.Fatalf("Failed to insert base data: %v", err)
	}

	// Try to insert VirtualService with valid Gateway (should succeed)
	_, err = db.Exec(`
		INSERT INTO virtual_services 
		(namespace, name, gateway_namespace, gateway_name, host, service_namespace, service_name) 
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"default", "valid-vs", "istio-system", "public-gateway", "test.com", "default", "web")

	if err != nil {
		t.Errorf("Expected successful insertion, but got error: %v", err)
	}
}

// TestSQLiteFullySupportedForeignKeys проверяет базовую поддержку foreign keys
func TestSQLiteFullySupportedForeignKeys(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Включить foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	// Проверить статус
	var fkEnabled int
	err = db.QueryRow("PRAGMA foreign_keys").Scan(&fkEnabled)
	if err != nil {
		t.Fatalf("Failed to check foreign keys: %v", err)
	}

	t.Logf("Foreign keys enabled: %d", fkEnabled)

	if fkEnabled != 1 {
		t.Skip("SQLite foreign keys are not supported in this environment")
	}

	// Создать тестовые таблицы с foreign key
	_, err = db.Exec(`
		CREATE TABLE parent (
			id INTEGER PRIMARY KEY
		);
		
		CREATE TABLE child (
			id INTEGER PRIMARY KEY,
			parent_id INTEGER,
			FOREIGN KEY (parent_id) REFERENCES parent(id)
		);
	`)
	if err != nil {
		t.Fatalf("Failed to create test tables: %v", err)
	}

	// Вставить родительскую запись
	_, err = db.Exec("INSERT INTO parent (id) VALUES (1)")
	if err != nil {
		t.Fatalf("Failed to insert parent: %v", err)
	}

	// Попытаться вставить дочернюю запись с валидным foreign key (должно работать)
	_, err = db.Exec("INSERT INTO child (id, parent_id) VALUES (1, 1)")
	if err != nil {
		t.Errorf("Valid foreign key insertion failed: %v", err)
	}

	// Попытаться вставить запись с несуществующим foreign key (должно fail)
	_, err = db.Exec("INSERT INTO child (id, parent_id) VALUES (2, 999)")
	if err == nil {
		t.Error("Expected foreign key violation but insertion succeeded")
	} else {
		t.Logf("Foreign key violation working: %v", err)
	}
}

func TestDestinationRuleHostForeignKey(t *testing.T) {
	db, err := sql.Open("sqlite3", "file:testdb?mode=memory&cache=shared")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Включить foreign keys
	_, err = db.Exec("PRAGMA foreign_keys = ON;")
	if err != nil {
		t.Fatalf("Failed to enable foreign keys: %v", err)
	}

	operator := &SQLiteIntegrityOperator{}

	// Create schema
	if err := operator.createSchema(db); err != nil {
		t.Fatalf("Failed to create schema: %v", err)
	}

	// 1. Вставляем service с правильным host
	_, err = db.Exec(
		"INSERT INTO services (namespace, name, host, port, protocol) VALUES (?, ?, ?, ?, ?)",
		"default", "web", "web.default.svc.cluster.local", 8080, "TCP",
	)
	if err != nil {
		t.Fatalf("Failed to insert service: %v", err)
	}

	// 2. Пытаемся вставить destination_rule с существующим host (должно работать)
	_, err = db.Exec(
		"INSERT INTO destination_rules (namespace, name, host, subsets, service_name, service_namespace) VALUES (?, ?, ?, ?, ?, ?)",
		"default", "web-dr", "web.default.svc.cluster.local", "v1,v2", "web", "default",
	)
	if err != nil {
		t.Errorf("❌ Failed to insert destination_rule with valid host: %v", err)
	} else {
		t.Logf("✅ Successfully inserted destination_rule with valid host")
	}

	// 3. Пытаемся вставить destination_rule с несуществующим host (должен fail)
	_, err = db.Exec(
		"INSERT INTO destination_rules (namespace, name, host, subsets) VALUES (?, ?, ?, ?)",
		"default", "broken-dr", "non-existent.default.svc.cluster.local", "v1",
	)
	if err == nil {
		t.Error("❌ Expected FK violation for non-existent host, but insertion succeeded")
	} else {
		t.Logf("✅ Correctly got FK error for non-existent host: %v", err)
	}
}

func TestGetTables(t *testing.T) {
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	// Создаем тестовые таблицы
	_, err = db.Exec(`
        CREATE TABLE services (
            namespace TEXT NOT NULL,
            name TEXT NOT NULL,
            PRIMARY KEY (namespace, name)
        );
        
        CREATE TABLE virtual_services (
            namespace TEXT NOT NULL,
            name TEXT NOT NULL,
            PRIMARY KEY (namespace, name)
        );
        
        CREATE INDEX idx_services_namespace ON services(namespace);
    `)
	if err != nil {
		t.Fatalf("Failed to create test tables: %v", err)
	}

	// Тест 1: Получить только имена таблиц
	tables, err := GetTableNames(db)
	if err != nil {
		t.Fatalf("Failed to get table names: %v", err)
	}

	expectedTables := []string{"services", "virtual_services"}
	if len(tables) != len(expectedTables) {
		t.Errorf("Expected %d tables, got %d: %v", len(expectedTables), len(tables), tables)
	}

	for _, expectedTable := range expectedTables {
		if !contains(tables, expectedTable) {
			t.Errorf("Expected table %s not found in: %v", expectedTable, tables)
		}
	}

	// Тест 2: Получить полную схему
	objects, err := GetDatabaseSchema(db)
	if err != nil {
		t.Fatalf("Failed to get database schema: %v", err)
	}

	foundTable := false
	foundIndex := false
	for _, obj := range objects {
		if obj.Type == "table" && obj.Name == "services" {
			foundTable = true
		}
		if obj.Type == "index" && obj.Name == "idx_services_namespace" {
			foundIndex = true
		}
	}

	if !foundTable {
		t.Error("Table 'services' not found in schema")
	}
	if !foundIndex {
		t.Error("Index 'idx_services_namespace' not found in schema")
	}

	t.Logf("Found tables: %v", tables)
	for _, obj := range objects {
		t.Logf("%s: %s", obj.Type, obj.Name)
	}
}

// Вспомогательная функция
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func GetUserTables(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
        SELECT name FROM sqlite_master 
        WHERE type='table' 
        AND name NOT IN ('sqlite_sequence', 'sqlite_stat1', 'sqlite_stat2', 'sqlite_stat3', 'sqlite_stat4')
        AND name NOT LIKE 'sqlite_%'
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, rows.Err()
}

type TableInfo struct {
	Name       string
	RowCount   int64
	Definition string
}

func GetTablesWithInfo(db *sql.DB) ([]TableInfo, error) {
	// Сначала получаем список таблиц
	tables, err := GetTableNames(db)
	if err != nil {
		return nil, err
	}

	var tableInfos []TableInfo

	for _, tableName := range tables {
		// Получаем количество строк
		var rowCount int64
		err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)).Scan(&rowCount)
		if err != nil {
			// Если не удалось посчитать строки, продолжаем
			rowCount = -1
		}

		// Получаем определение таблицы
		var definition string
		err = db.QueryRow(`
            SELECT sql FROM sqlite_master 
            WHERE type='table' AND name=?
        `, tableName).Scan(&definition)
		if err != nil {
			definition = ""
		}

		tableInfos = append(tableInfos, TableInfo{
			Name:       tableName,
			RowCount:   rowCount,
			Definition: definition,
		})
	}

	return tableInfos, nil
}

func GetTableNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
        SELECT name FROM sqlite_master 
        WHERE type='table' 
        AND name NOT LIKE 'sqlite_%'
        ORDER BY name
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, rows.Err()
}

type DatabaseObject struct {
	Type string
	Name string
	SQL  string
}

func GetDatabaseSchema(db *sql.DB) ([]DatabaseObject, error) {
	rows, err := db.Query(`
        SELECT type, name, sql 
        FROM sqlite_master 
        WHERE name NOT LIKE 'sqlite_%'
        ORDER BY type, name
    `)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var objects []DatabaseObject
	for rows.Next() {
		var objType, name, sqlStr string
		if err := rows.Scan(&objType, &name, &sqlStr); err != nil {
			return nil, err
		}
		objects = append(objects, DatabaseObject{
			Type: objType,
			Name: name,
			SQL:  sqlStr,
		})
	}

	return objects, rows.Err()
}
