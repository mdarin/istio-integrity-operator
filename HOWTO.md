Пример полного процесса

Установка и запуск:

bash
# Инициализация
make deps
make manifests
make generate

# Локальный запуск
make run-with-sqlite

# Сборка и деплой
make docker-build docker-push
make deploy

1. Сборка и установка:

bash
# Сгенерировать манифесты
make manifests

# Установить CRD
make install

# Проверить CRD
kubectl get crd | grep meshservice

# Собрать и запустить контроллер локально (для разработки)
make run

# ИЛИ установить контроллер в кластер
make docker-build docker-push IMG=your-registry/istio-operator:v1.0.0
make deploy IMG=your-registry/istio-operator:v1.0.0
2. Проверка работы:

bash
# Проверить, что контроллер запущен
kubectl get pods -n istio-operator-system

# Проверить логи контроллера
kubectl logs -n istio-operator-system deployment/istio-operator-controller-manager

# Применить пример MeshService
kubectl apply -f config/samples/mesh_v1alpha1_meshservice.yaml

# Проверить созданный MeshService
kubectl get meshservices.mesh.istio.operator

# Проверить статус
kubectl describe meshservice sample-meshservice