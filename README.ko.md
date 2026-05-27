# Mobilint Device Plugin

[English](README.md) | **한국어**

Kubernetes에서 Mobilint NPU(`/dev/aries[0-9]*`)를 `mobilint.com/npu` 리소스로 노출하는 Device Plugin입니다.  
DaemonSet으로 NPU 노드마다 한 개씩 동작하며, kubelet에 NPU 개수를 등록하고,  
Pod이 요청 시 `/dev/ariesN` 디바이스 노드와 `MOBILINT_VISIBLE_DEVICES` 환경변수를 컨테이너에 주입합니다.

## 시스템 요구사항

| 항목 | 요구사항 |
|---|---|
| Kubernetes | device plugin v1beta1 API를 지원하는 버전 |
| 노드 OS | Linux (Ubuntu 권장) |
| 컨테이너 런타임 | device cgroup 지원되는 CRI 런타임 |
| 커널 드라이버 | Aries 드라이버 |

NPU 카드가 박힌 노드에서 다음이 모두 만족돼야 합니다:
```bash
lsmod | grep aries           # 드라이버 모듈 로드 확인
ls /dev/aries*               # 디바이스 노드 존재 확인
```

드라이버가 설치되어있지 않은 경우 [docs.mobilint.com](https://docs.mobilint.com)의 "Installing driver" 섹션을 참고해서 설치하세요.

## 설치

### 1. NPU 노드에 라벨 부여
```bash
kubectl label node <NODE_NAME> mobilint.com/npu.present=true --overwrite
```

### 2. DaemonSet 배포

```bash
kubectl apply -f deploy/daemonset.yaml
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

### 3) 테스트 Pod 실행

`deploy/test-pod.yaml`은 NPU 1개를 요청하는 최소 Pod입니다.

```bash
kubectl apply -f deploy/test-pod.yaml
kubectl wait --for=condition=Ready pod/mobilint-npu-test --timeout=60s
kubectl logs mobilint-npu-test
```

정상 출력 예 (실 SDK 포함 이미지 사용 시):
```
MOBILINT_VISIBLE_DEVICES=aries0
crw-rw-rw- 1 root root 503, 0 ... /dev/aries0
mobilint-cli        1.2.0
mobilint-qb-runtime 1.2.0
/dev/aries0         Aries
```

정리:
```bash
kubectl delete -f deploy/test-pod.yaml
```

## 워크로드에서 NPU 사용

Pod이 `mobilint.com/npu` 리소스를 요청하면 컨테이너에 다음이 주입됩니다:

- `/dev/aries<N>` — Allocate된 NPU의 character device (rw)
- `MOBILINT_VISIBLE_DEVICES=aries<N>[,aries<M>...]` — 사용 가능한 NPU id 목록

## 제거

### 플러그인 및 노드라벨 제거
```bash
kubectl delete -f deploy/daemonset.yaml
kubectl label node <NODE_NAME> mobilint.com/npu.present-
```
