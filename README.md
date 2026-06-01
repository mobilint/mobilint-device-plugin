# Mobilint Device Plugin

**English** | [한국어](README.ko.md)

A Kubernetes device plugin that exposes Mobilint NPUs (`/dev/aries[0-9]*`) as the `mobilint.com/npu` resource.

## System Requirements

| Item | Requirement |
|---|---|
| Kubernetes | 1.31+ |
| Node OS | Linux (Ubuntu recommended) |
| Container runtime | A CDI-enabled CRI runtime: containerd ≥ 1.7 or CRI-O ≥ 1.23 |
| Kernel driver | Aries driver |

On every node with an NPU card, the following must hold:
```bash
lsmod | grep aries           # kernel module is loaded
ls /dev/aries*               # device nodes exist
```

If the driver is not installed, follow the "Installing driver" section on [docs.mobilint.com](https://docs.mobilint.com) first.

This project injects device nodes through the Container Device Interface (CDI), so CDI must be enabled on the container runtime.

## Installation

### 1. Label NPU nodes
```bash
kubectl label node <NODE_NAME> mobilint.com/npu.present=true --overwrite
```

On larger or autoscaled clusters you can skip manual labeling and let Node Feature Discovery do it automatically — see [Automatic node labeling with NFD](#automatic-node-labeling-with-nfd-optional).

### 2. Deploy the plugin

**With Helm (recommended):**
```bash
helm install mobilint-device-plugin \
  oci://ghcr.io/mobilint/charts/mobilint-device-plugin --version 0.2.0 -n kube-system
```
See [chart/values.yaml](chart/values.yaml) for configurable options (image tag, metrics Service/ServiceMonitor, NetworkPolicy, kubelet path, etc.). From a local checkout, replace the `oci://` URL with `./chart`.

**Without Helm:**
```bash
kubectl apply -f https://raw.githubusercontent.com/mobilint/mobilint-device-plugin/master/deploy/daemonset.yaml
```

> `deploy/*.yaml` is generated from the Helm chart (`make manifests`) — edit the chart, not these files.

## Automatic node labeling with NFD (optional)

Instead of labeling nodes by hand, [NFD](https://github.com/kubernetes-sigs/node-feature-discovery) can automatically apply `mobilint.com/npu.present=true` to nodes that have an NPU.

```bash
# 1. Install NFD (provides the NodeFeatureRule CRD), allowing the mobilint.com label namespace
helm repo add nfd https://kubernetes-sigs.github.io/node-feature-discovery/charts
helm install nfd nfd/node-feature-discovery -n node-feature-discovery --create-namespace \
  --set master.extraLabelNs={mobilint.com}

# 2. Enable auto-labeling in this chart
helm install mobilint-device-plugin oci://ghcr.io/mobilint/charts/mobilint-device-plugin \
  --version 0.2.0 -n kube-system --set nodeFeatureDiscovery.enabled=true
```

## Verification

### 1) Plugin Pod readiness

```bash
kubectl -n kube-system get pods -l app.kubernetes.io/name=mobilint-device-plugin -o wide
```
You should see one `READY 1/1` Pod per labeled NPU node.

### 2) Confirm the node has the NPU resource registered

```bash
kubectl get node <NODE_NAME> -o jsonpath='{.status.allocatable.mobilint\.com/npu}{"\n"}'
```
The NPU count should be printed (`1`, `2`, ...).

## Using the NPU in a Pod

Request `mobilint.com/npu` under `resources.limits`:

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

```bash
kubectl logs npu-example
# crw-rw-rw- 1 root root 503, 0 ... /dev/aries0
```

> To actually run inference, the workload image must include the Mobilint SDK/runtime — see [docs.mobilint.com](https://docs.mobilint.com).

## Uninstall

### Remove the plugin and node label
```bash
# Helm
helm uninstall mobilint-device-plugin -n kube-system
# if installed from the raw manifest
kubectl delete -f https://raw.githubusercontent.com/mobilint/mobilint-device-plugin/master/deploy/daemonset.yaml

kubectl label node <NODE_NAME> mobilint.com/npu.present-
```

## License

[Apache License 2.0](LICENSE) © Mobilint, Inc.
