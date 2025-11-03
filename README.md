# istio-integrity-operator

## Istio Mesh Service Operator

A Kubernetes Operator that automates and simplifies Istio service mesh configuration management using SQLite in-memory referential integrity checks.

## 🚀 Overview

The Istio Mesh Service Operator provides a declarative way to manage Istio resources while ensuring consistency and integrity across your service mesh configuration. It eliminates common configuration drift issues and enforces relational integrity between Istio resources.

## ✨ Features

- **Declarative Service Mesh Management** - Define your service mesh configuration using custom Kubernetes resources
- **Automated Integrity Checks** - Built-in SQLite in-memory database for referential integrity validation
- **Consistency Enforcement** - Automatically detects and repairs configuration drift
- **Multi-Resource Coordination** - Manages VirtualServices, Gateways, and Services as a single unit
- **Cross-Namespace Support** - Maintains consistency across different Kubernetes namespaces

## 🛠 How It Works

```yaml
apiVersion: mesh.istio.operator/v1alpha1
kind: MeshService
metadata:
  name: webapp-service
spec:
  serviceName: webapp
  hosts:
    - "webapp.example.com"
  gateway:
    name: public-gateway
    namespace: istio-system
  ports:
    - port: 80
      targetPort: 8080

## Description

The operator automatically:

✅ Creates and manages all related Istio resources
✅ Validates referential integrity using in-memory SQLite
✅ Maintains consistency across namespaces
✅ Provides detailed status and health information
📦 Installation

```bash
# Deploy the operator
kubectl apply -f config/deploy/

# Create your first MeshService
kubectl apply -f config/samples/meshservice.yaml
```

🔍 Integrity Checking

The operator uses SQLite in-memory database to perform relational integrity checks:

Foreign key validation between VirtualServices and Gateways
Unique constraint checks for host/port combinations
Automatic repair plans for inconsistent states
Real-time consistency reporting

🏗 Use Cases

Multi-team environments - Ensure consistent Istio configuration across teams
GitOps workflows - Maintain configuration integrity in CI/CD pipelines
Large-scale meshes - Prevent configuration drift in complex service meshes
Compliance requirements - Enforce strict configuration standards

📄 Documentation

[Quick Start Guide]()

[API Reference]()

[Examples]()

🤝 Contributing

We welcome contributions! Please see our [Contributing Guide]() for details.

📄 License

This project is licensed under the Apache 2.0 License - see the LICENSE file for details.

## Getting Started

### Prerequisites
- go version v1.23.0+
- docker version 17.03+.
- kubectl version v1.11.3+.
- Access to a Kubernetes v1.11.3+ cluster.

### To Deploy on the cluster
**Build and push your image to the location specified by `IMG`:**

```sh
make docker-build docker-push IMG=<some-registry>/istio-integrity-operator:tag
```

**NOTE:** This image ought to be published in the personal registry you specified.
And it is required to have access to pull the image from the working environment.
Make sure you have the proper permission to the registry if the above commands don’t work.

**Install the CRDs into the cluster:**

```sh
make install
```

**Deploy the Manager to the cluster with the image specified by `IMG`:**

```sh
make deploy IMG=<some-registry>/istio-integrity-operator:tag
```

> **NOTE**: If you encounter RBAC errors, you may need to grant yourself cluster-admin
privileges or be logged in as admin.

**Create instances of your solution**
You can apply the samples (examples) from the config/sample:

```sh
kubectl apply -k config/samples/
```

>**NOTE**: Ensure that the samples has default values to test it out.

### To Uninstall
**Delete the instances (CRs) from the cluster:**

```sh
kubectl delete -k config/samples/
```

**Delete the APIs(CRDs) from the cluster:**

```sh
make uninstall
```

**UnDeploy the controller from the cluster:**

```sh
make undeploy
```

## Project Distribution

Following the options to release and provide this solution to the users.

### By providing a bundle with all YAML files

1. Build the installer for the image built and published in the registry:

```sh
make build-installer IMG=<some-registry>/istio-integrity-operator:tag
```

**NOTE:** The makefile target mentioned above generates an 'install.yaml'
file in the dist directory. This file contains all the resources built
with Kustomize, which are necessary to install this project without its
dependencies.

2. Using the installer

Users can just run 'kubectl apply -f <URL for YAML BUNDLE>' to install
the project, i.e.:

```sh
kubectl apply -f https://raw.githubusercontent.com/<org>/istio-integrity-operator/<tag or branch>/dist/install.yaml
```

### By providing a Helm Chart

1. Build the chart using the optional helm plugin

```sh
kubebuilder edit --plugins=helm/v1-alpha
```

2. See that a chart was generated under 'dist/chart', and users
can obtain this solution from there.

**NOTE:** If you change the project, you need to update the Helm Chart
using the same command above to sync the latest changes. Furthermore,
if you create webhooks, you need to use the above command with
the '--force' flag and manually ensure that any custom configuration
previously added to 'dist/chart/values.yaml' or 'dist/chart/manager/manager.yaml'
is manually re-applied afterwards.

## Contributing
// TODO(user): Add detailed information on how you would like others to contribute to this project

**NOTE:** Run `make help` for more information on all potential `make` targets

More information can be found via the [Kubebuilder Documentation](https://book.kubebuilder.io/introduction.html)

## License

Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
