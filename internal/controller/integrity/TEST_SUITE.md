# Модульные тесты для ядра системы integrity, которые можно запускать без всего оператора

Для запуска тестов:

```bash
# Запуск всех тестов integrity
go test ./internal/integrity/... -v

# Запуск только интеграционных тестов
go test ./internal/integrity/... -v -run="Integration"

# Запуск с покрытием
go test ./internal/integrity/... -cover -coverprofile=coverage.out
go tool cover -html=coverage.out
```

Эти тесты позволяют:

✅ Тестировать ядро системы без Kubernetes
✅ Проверять логику целостности изолированно
✅ Быстро итерироваться при разработке
✅ Интегрировать в CI/CD пайплайны
✅ Тестировать edge cases и ошибки
