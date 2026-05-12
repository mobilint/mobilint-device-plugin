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
# 실 클러스터: 레지스트리에 push 후 deploy/daemonset.yaml 의 image 수정
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
kubectl -n kube-system logs -l app=mobilint-device-plugin --tail=50
```
다음 로그가 보여야 한다:
- `device plugin server started socket=...`
- `discovered device id=ariesN ... health=Healthy`
- `registered mobilint.com/npu with kubelet`

### 5. 리소스 광고 확인
```bash
kubectl get node <NODE> -o jsonpath='{.status.allocatable.mobilint\.com/npu}{"\n"}'
```
NPU 개수가 출력되어야 한다.

### 6. 할당 테스트
```bash
kubectl apply -f deploy/test-pod.yaml
kubectl logs mobilint-npu-test
```
출력 예:
```
MOBILINT_VISIBLE_DEVICES=aries0
crw-rw---- 1 root root ... /dev/aries0
```

### 7. 정리
```bash
kubectl delete -f deploy/test-pod.yaml
kubectl delete -f deploy/daemonset.yaml
```
