# Mobilint Device Plugin

**English** | [한국어](README.ko.md)

A Kubernetes Device Plugin that exposes Mobilint NPUs (`/dev/aries[0-9]*`) as the `mobilint.com/npu` resource.
It runs as a DaemonSet, one Pod per NPU node, registers the NPU count with kubelet,
and on Pod allocation injects the matching `/dev/ariesN` device node and the `MOBILINT_VISIBLE_DEVICES` environment variable into the container.

## System Requirements

| Item | Requirement |
|---|---|
| Kubernetes | A version that supports the device plugin v1beta1 API |
| Node OS | Linux (Ubuntu recommended) |
| Container runtime | A CRI runtime with device cgroup support |
| Kernel driver | Aries driver |

On every node with an NPU card, the following must hold:
```bash
lsmod | grep aries           # kernel module is loaded
ls /dev/aries*               # device nodes exist
```

If the driver is not installed, follow the "Installing driver" section on [docs.mobilint.com](https://docs.mobilint.com) first.

## Installation

### 1. Label NPU nodes
```bash
kubectl label node <NODE_NAME> mobilint.com/npu.present=true --overwrite
```

### 2. Deploy the DaemonSet

```bash
kubectl apply -f deploy/daemonset.yaml
```

## Verification

### 1) Plugin Pod readiness

```bash
kubectl -n kube-system get pods -l app=mobilint-device-plugin -o wide
```
You should see one `READY 1/1` Pod per labeled NPU node.

### 2) Confirm the node has the NPU resource registered

```bash
kubectl get node <NODE_NAME> -o jsonpath='{.status.allocatable.mobilint\.com/npu}{"\n"}'
```
The NPU count should be printed (`1`, `2`, ...).

### 3) Run the test Pod

`deploy/test-pod.yaml` is a minimal Pod that requests one NPU and uses the test image from `deploy/test.Dockerfile`.

```bash
docker build -f deploy/test.Dockerfile -t mobilint-npu-test:v0.1.0 .
# Load into your cluster (k3s example; for other runtimes use ctr -n k8s.io / kind load / minikube image load)
docker save mobilint-npu-test:v0.1.0 | sudo k3s ctr images import -
kubectl apply -f deploy/test-pod.yaml
kubectl wait --for=condition=Ready pod/mobilint-npu-test --timeout=60s
kubectl logs mobilint-npu-test
```

Expected output (when using an image with the Mobilint SDK preinstalled):
```
MOBILINT_VISIBLE_DEVICES=aries0
crw-rw-rw- 1 root root 503, 0 ... /dev/aries0
mobilint-cli        1.2.0
mobilint-qb-runtime 1.2.0
/dev/aries0         Aries
```

Clean up:
```bash
kubectl delete -f deploy/test-pod.yaml
```

## Using the NPU in your workloads

When a Pod requests `mobilint.com/npu`, the following is injected into the container:

- `/dev/aries<N>` — the allocated NPU's character device (rw)
- `MOBILINT_VISIBLE_DEVICES=aries<N>[,aries<M>...]` — comma-separated list of usable NPU ids

## Uninstall

### Remove the plugin and node label
```bash
kubectl delete -f deploy/daemonset.yaml
kubectl label node <NODE_NAME> mobilint.com/npu.present-
```
