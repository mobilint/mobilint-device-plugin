# Mobilint Device Plugin

**English** | [한국어](README.ko.md)

Mobilint Device Plugin is a Kubernetes device plugin that exposes ARIES NPUs as a schedulable resource (`mobilint.com/npu`) in Kubernetes.

## Documentation
For detailed guides on installation, operation, NFD integration, and troubleshooting, see the documentation below.  
https://docs.mobilint.com

## Overview

Once the plugin is installed, the node's ARIES devices (`/dev/aries*`) are registered as Kubernetes resources.

```yaml
resources:
  limits:
    mobilint.com/npu: 1
```

## Requirements

- Kubernetes 1.31+
- Linux
- CDI-enabled container runtime
  - containerd 1.7+
  - CRI-O 1.23+
- ARIES Driver

For driver installation, see the Mobilint documentation.

## Installation

Add a label to the NPU nodes.

```bash
kubectl label node <NODE_NAME> mobilint.com/npu.present=true --overwrite
```

Install the Device Plugin with Helm.

```bash
helm install mobilint-device-plugin \
  oci://ghcr.io/mobilint/charts/mobilint-device-plugin \
  -n kube-system
```

## Usage Example

To request an NPU in a Pod, specify the `mobilint.com/npu` resource.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: npu-example
spec:
  containers:
    - name: app
      image: ubuntu:latest
      command: ["sh", "-c", "ls -l /dev/aries*; sleep infinity"]
      resources:
        limits:
          mobilint.com/npu: 1
```

## License

Apache License 2.0 © Mobilint, Inc.