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

---

## Prometheus 메트릭

플러그인은 컨테이너 포트 **9400**에서 `/metrics` (Prometheus 텍스트 포맷)와 `/healthz`를 제공한다.

### 노출 메트릭

| 메트릭 | 단위 | 출처 |
|---|---|---|
| `mobilint_npu_health` | 0/1 | `ARIES_IOC_DRIVER_INFO` 응답 여부 |
| `mobilint_npu_temperature` | 드라이버 raw | `ARIES_IOC_GET_TEMPERATURE` |
| `mobilint_npu_clock_npu_hz` | Hz | `ARIES_IOC_GET_CLOCK_NPU` |
| `mobilint_npu_clock_noc_hz` | Hz | `ARIES_IOC_GET_CLOCK_NOC` |
| `mobilint_npu_power_total` | 드라이버 raw | `ARIES_IOC_GET_TOTAL_POWER` |
| `mobilint_npu_current_total` | 드라이버 raw | `ARIES_IOC_GET_TOTAL_CURRENT` |
| `mobilint_npu_voltage_total` | 드라이버 raw | `ARIES_IOC_GET_TOTAL_VOLTAGE` |
| `mobilint_npu_fan_duty` | % (추정) | `ARIES_IOC_GET_FAN_DUTY` |
| `mobilint_npu_fd_count` | 개 | `ARIES_IOC_GET_FD_COUNT` |

모든 메트릭은 `device="ariesN"` 라벨을 갖는다. raw 값은 PromQL로 적절히 스케일링하여 사용.

### 빠른 확인
```bash
POD=$(kubectl -n kube-system get pod -l app=mobilint-device-plugin -o jsonpath='{.items[0].metadata.name}')
kubectl -n kube-system port-forward "$POD" 9400:9400 &
curl -s http://localhost:9400/metrics | grep mobilint_npu
```

### Prometheus 스크레이프 (Pod IP 직접)

```yaml
# prometheus.yml scrape_configs 발췌
- job_name: mobilint-device-plugin
  kubernetes_sd_configs:
    - role: pod
      namespaces:
        names: [kube-system]
  relabel_configs:
    - source_labels: [__meta_kubernetes_pod_label_app]
      regex: mobilint-device-plugin
      action: keep
    - source_labels: [__meta_kubernetes_pod_container_port_name]
      regex: metrics
      action: keep
    - source_labels: [__meta_kubernetes_pod_node_name]
      target_label: node
```

ServiceMonitor를 쓰려면 `deploy/`에 headless Service를 추가하면 된다.

---

## 워크로드 컨테이너에 SDK 전달

본 device plugin은 디바이스 파일(`/dev/ariesN`) + 환경변수(`MOBILINT_VISIBLE_DEVICES`) 만 제공한다.
실제 inference를 위한 **libmobilint SDK 전달은 워크로드 측 책임**이며, 다음 두 패턴 중 선택한다.

### 패턴 A. 호스트 SDK를 hostPath로 마운트

노드의 `/opt/mobilint` 에 SDK가 사전 설치되어 있다고 가정.

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: inference-host-sdk
spec:
  containers:
    - name: app
      image: my-app:slim                    # SDK 미포함 슬림 이미지
      env:
        - name: LD_LIBRARY_PATH
          value: /opt/mobilint/lib
      resources:
        limits:
          mobilint.com/npu: 1
      volumeMounts:
        - name: mobilint-sdk
          mountPath: /opt/mobilint
          readOnly: true
  volumes:
    - name: mobilint-sdk
      hostPath:
        path: /opt/mobilint
        type: Directory
```

장점: 이미지 작음, SDK 업데이트 시 노드만 갱신하면 모든 워크로드 반영.
조건: 노드 프로비저닝 시 SDK 설치 필요.

### 패턴 B. 사용자 이미지에 SDK 번들

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: inference-bundled-sdk
spec:
  containers:
    - name: app
      image: my-app-with-sdk:v1.0           # 이미지 빌드 시 libmobilint 포함
      resources:
        limits:
          mobilint.com/npu: 1
```

장점: 호스트 의존 없음, hermetic, SDK 버전이 이미지 태그로 고정.
조건: SDK 업데이트 시 이미지 재빌드.

### 동시 운영

두 패턴은 **동일 클러스터/노드에서 공존** 가능하다. Plugin은 어느 쪽이든 동일하게 디바이스만 할당한다. 워크로드별로 자유롭게 선택하면 된다.
