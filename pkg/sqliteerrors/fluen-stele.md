# Использование в коде:

Fluent style:

```go
import "github.com/your-org/istio-integrity-operator/pkg/sqliteerrors"

// В loadData
if _, err := tx.Exec(query, args...); err != nil {
    analyzer := sqliteerrors.E(err)
    
    if analyzer.Syntax().Is() {
        // Критическая ошибка - прерываем
        return nil, fmt.Errorf("syntax error: %v", err)
    }
    
    if analyzer.ForeignKey().Is() {
        // Штатная FK violation - добавляем в отчет
        report.Violations = append(report.Violations, ...)
        continue // продолжаем загрузку
    }
    
    if analyzer.Busy().Is() || analyzer.Locked().Is() {
        // Retry-able ошибки
        return nil, fmt.Errorf("temporary error: %v", err)
    }
    
    // Неизвестная ошибка - считаем критической
    return nil, fmt.Errorf("unexpected error: %v", err)
}
```


Helper functions style:
```go
import "github.com/your-org/istio-integrity-operator/pkg/sqliteerrors"

if _, err := tx.Exec(query, args...); err != nil {
    if sqliteerrors.IsSyntax(err) {
        return nil, fmt.Errorf("syntax error: %v", err)
    }

    if sqliteerrors.IsForeignKey(err) {
        // Обработка FK violation
        report.Violations = append(report.Violations, ...)
        continue
    }
    
    if sqliteerrors.ShouldAbort(err) {
        return nil, fmt.Errorf("critical error: %v", err)
    }
}
```
