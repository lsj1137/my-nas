# 로컬 개발 환경

운영 서버와 같은 구성(Nginx → 게이트웨이 → Quantum)을 내 PC에서 띄워서, 배포하지 않고 확인한다.
필요한 것: Docker Desktop, Git Bash.

## 처음 한 번

```bash
bash dev/setup.sh
docker compose -f dev/docker-compose.yml up -d --build
```

브라우저에서 **https://nas.localhost:8443** 접속.
자체 서명 인증서라 처음에 경고가 뜬다 → "고급" → "nas.localhost(으)로 이동".
(`*.localhost` 주소는 브라우저가 자동으로 내 PC로 연결하므로 hosts 파일 수정은 필요 없다)

로그인 계정: `owner` / `dev-password`

## 무엇을 고쳤을 때 무엇을 하나

| 고친 것 | 할 일 |
|---|---|
| `gateway/web/` 아래 HTML, CSS, JS | **저장 후 새로고침**만 하면 됨 (게이트웨이가 디스크에서 바로 읽음) |
| 게이트웨이 Go 코드 | `docker compose -f dev/docker-compose.yml up -d --build nas-gateway` |
| `dev/gateway.yaml` (IP 매핑, 계정) | `docker compose -f dev/docker-compose.yml kill -s HUP nas-gateway` |
| `nginx/`, `filebrowser/config.example.yaml` (운영 설정) | `bash dev/setup.sh` 후 `docker compose -f dev/docker-compose.yml restart` |

## 상황별 테스트

- **집 와이파이 상황 (IP 자동 접속)**: 기본값. 접속하면 바로 `shared`로 들어간다.
- **외부 접속 상황 (로그인 필요)**: `dev/gateway.yaml`의 `ip_map` 항목을 주석 처리하고 HUP.

## E2E 테스트 (실제 브라우저로 자동 확인)

설치된 Chrome을 자동으로 조작해서 계정 전환, 삭제 경고, 암호화 보관함, 로그아웃까지 확인한다.
화면을 고친 뒤 기존 기능이 깨지지 않았는지 볼 때 쓴다. (`ip_map`이 켜진 기본 상태에서 실행)

```bash
cd dev/e2e
npm install      # 처음 한 번
npm test
```

## 정리

```bash
docker compose -f dev/docker-compose.yml down        # 중지
rm -rf dev/.generated                                # 테스트 파일/DB까지 초기화
```
