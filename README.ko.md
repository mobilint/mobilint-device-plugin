# Mobilint Device Plugin

[English](README.md) | **한국어**

Mobilint Device Plugin은 Kubernetes에서 ARIES NPU를 스케줄 가능한 리소스(`mobilint.com/npu`)로 노출하는 Kubernetes Device Plugin입니다.

## 문서
설치, 운영, NFD 연동, 문제 해결 등 상세 가이드는 아래 문서를 참고하세요.  
https://docs.mobilint.com

## 개요

플러그인을 설치하면 노드의 ARIES 디바이스(`/dev/aries*`)가 Kubernetes 리소스로 등록됩니다.

```yaml
resources:
  limits:
    mobilint.com/npu: 1
```

## 요구사항

- Kubernetes 1.31+
- Linux
- CDI 지원 컨테이너 런타임
  - containerd 1.7+
  - CRI-O 1.23+
- ARIES Driver

드라이버 설치 방법은 Mobilint 문서를 참고하세요.

## 설치

NPU 노드에 라벨을 추가합니다.

```bash
kubectl label node <NODE_NAME> mobilint.com/npu.present=true --overwrite
```

Helm으로 Device Plugin을 설치합니다.

```bash
helm install mobilint-device-plugin \
  oci://ghcr.io/mobilint/charts/mobilint-device-plugin \
  -n kube-system
```

## 사용 예제

Pod에서 NPU를 요청하려면 `mobilint.com/npu` 리소스를 지정합니다.

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