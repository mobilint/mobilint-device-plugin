# Mobilint Device Plugin

[English](README.md) | **한국어**

Kubernetes에서 Mobilint NPU(`/dev/aries[0-9]*`)를 `mobilint.com/npu` 리소스로 노출하는 Device Plugin입니다.

## 시스템 요구사항

| 항목 | 요구사항 |
|---|---|
| Kubernetes | 1.31+ |
| 노드 OS | Linux (Ubuntu 권장) |
| 컨테이너 런타임 | CDI 지원 CRI 런타임: containerd ≥ 1.7 또는 CRI-O ≥ 1.23 |
| 커널 드라이버 | Aries 드라이버 |

NPU 카드가 박힌 노드에서 다음이 모두 만족돼야 합니다:
```bash
lsmod | grep aries           # 드라이버 모듈 로드 확인
ls /dev/aries*               # 디바이스 노드 인식 확인
```

드라이버가 설치되어있지 않은 경우 [docs.mobilint.com](https://docs.mobilint.com)의 "Installing driver" 섹션을 참고해서 설치하세요.

이 프로젝트는 CDI(Container Device Interface)로 디바이스 노드를 주입하므로, 컨테이너 런타임에 CDI가 활성화돼 있어야 합니다.

## 설치

### 1. NPU 노드에 라벨 부여
```bash
kubectl label node <NODE_NAME> mobilint.com/npu.present=true --overwrite
```

노드가 많거나 오토스케일링 환경이면 수동 라벨링 대신 Node Feature Discovery로 자동화할 수 있습니다 — [NFD로 자동 노드 라벨링](#nfd로-자동-노드-라벨링-선택) 참고.

### 2. 플러그인 배포

**Helm 사용 (권장):**
```bash
helm install mobilint-device-plugin \
  oci://ghcr.io/mobilint/charts/mobilint-device-plugin --version 0.2.0 -n kube-system
```
설정 가능한 옵션(이미지 태그, metrics Service/ServiceMonitor, NetworkPolicy, kubelet 경로 등)은 [chart/values.yaml](chart/values.yaml) 참고. 로컬 체크아웃에서는 `oci://` URL 대신 `./chart`를 쓰면 됩니다.

**Helm 없이:**
```bash
kubectl apply -f https://raw.githubusercontent.com/mobilint/mobilint-device-plugin/master/deploy/daemonset.yaml
```

> `deploy/*.yaml`은 Helm 차트에서 생성됩니다(`make manifests`) — 이 파일이 아니라 차트를 수정하세요.

## NFD로 자동 노드 라벨링 (선택)

수동 라벨링 대신 [NFD](https://github.com/kubernetes-sigs/node-feature-discovery)를 사용해 NPU가 있는 노드에 `mobilint.com/npu.present=true`를 자동으로 붙일 수 있습니다.

```bash
# 1. NFD 설치 (NodeFeatureRule CRD 제공), mobilint.com 라벨 네임스페이스 허용
helm repo add nfd https://kubernetes-sigs.github.io/node-feature-discovery/charts
helm install nfd nfd/node-feature-discovery -n node-feature-discovery --create-namespace \
  --set master.extraLabelNs={mobilint.com}

# 2. 이 차트에서 자동 라벨링 켜기
helm install mobilint-device-plugin oci://ghcr.io/mobilint/charts/mobilint-device-plugin \
  --version 0.2.0 -n kube-system --set nodeFeatureDiscovery.enabled=true
```

## 검증

### 1) Plugin Pod Ready 여부

```bash
kubectl -n kube-system get pods -l app=mobilint-device-plugin -o wide
```
NPU 라벨이 붙은 노드 수만큼 Pod이 `READY 1/1`로 떠야 합니다.

### 2) 노드의 NPU 리소스 등록 확인

```bash
kubectl get node <NODE_NAME> -o jsonpath='{.status.allocatable.mobilint\.com/npu}{"\n"}'
```
NPU 개수가 출력돼야 합니다(`1`, `2`, ...).

## Pod에서 NPU 사용

`resources.limits`에 `mobilint.com/npu`를 요청합니다:

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

> 실제 추론을 돌리려면 워크로드 이미지에 Mobilint SDK/런타임이 포함돼야 합니다 — [docs.mobilint.com](https://docs.mobilint.com) 참고.

## 제거

### 플러그인 및 노드라벨 제거
```bash
# Helm
helm uninstall mobilint-device-plugin -n kube-system
# 또는 Helm 없이
kubectl delete -f deploy/daemonset.yaml

kubectl label node <NODE_NAME> mobilint.com/npu.present-
```

## 라이선스

[Apache License 2.0](LICENSE) © Mobilint, Inc.
