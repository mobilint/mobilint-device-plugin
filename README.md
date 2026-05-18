# Mobilint Device Plugin

Kubernetes Device Plugin for Mobilint NPU (`/dev/aries[0-9]*`).
노드의 NPU를 `mobilint.com/npu` 리소스로 광고한다.

---

## 테스트 절차

### 1. 이미지 빌드
```bash
docker build -t mobilint-device-plugin:v0.1.0 .
```

클러스터에 적재:
```bash
# k3s (containerd import)
docker save mobilint-device-plugin:v0.1.0 | sudo k3s ctr images import -
# kind
kind load docker-image mobilint-device-plugin:v0.1.0
# minikube
minikube image load mobilint-device-plugin:v0.1.0
```

### 2. NPU 노드 라벨링
```bash
kubectl get nodes
kubectl label node <NODE> mobilint.com/npu.present=true
```

### 3. DaemonSet 배포
```bash
kubectl apply -f deploy/daemonset.yaml
kubectl -n kube-system get pods -l app=mobilint-device-plugin -o wide
```

### 4. 로그 확인
```bash
kubectl -n kube-system logs -l app=mobilint-device-plugin
```
다음 로그가 보여야 합니다:
- `device plugin server started socket=...`
- `discovered devices: ariesN=Healthy`
- `metrics server listening addr=:9400`
- `registered mobilint.com/npu with kubelet`

### 5. 리소스 광고 확인
```bash
kubectl get node <NODE> -o jsonpath='{.status.allocatable.mobilint\.com/npu}{"\n"}'
```
NPU 개수가 출력되어야 합니다.

### 6. Runtime 포함 테스트
`mobilint-qb-runtime`과 `mobilint-cli`가 포함된 테스트 이미지를 빌드한다.

```bash
docker build -f deploy/test.Dockerfile -t mobilint-npu-test:v0.1.0 .
```

클러스터에 적재:
```bash
# k3s (containerd import)
docker save mobilint-npu-test:v0.1.0 | sudo k3s ctr images import -
# kind
kind load docker-image mobilint-npu-test:v0.1.0
# minikube
minikube image load mobilint-npu-test:v0.1.0
```

테스트 Pod를 실행한다. 이 테스트는 NPU 리소스 할당, `/dev/ariesN` 주입,
`MOBILINT_VISIBLE_DEVICES` 설정, runtime/CLI 동작을 함께 확인한다.
```bash
kubectl apply -f deploy/test-pod.yaml
kubectl logs mobilint-npu-test
```

출력 예:
```text
MOBILINT_VISIBLE_DEVICES=aries0
crw-rw-rw- 1 root root ... /dev/aries0
mobilint-cli        1.2.0
mobilint-qb-runtime 1.2.0
/dev/aries0         Aries
```

실제 추론 워크로드는 이 이미지처럼 runtime을 포함한 이미지에서 실행하거나,
노드에 설치된 SDK/runtime 경로를 `hostPath`로 마운트해야 한다.

### 7. 정리
```bash
kubectl delete -f deploy/test-pod.yaml
kubectl delete -f deploy/daemonset.yaml
```
