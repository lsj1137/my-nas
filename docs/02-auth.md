# 02. 인증 설계

## 판단 순서

Nginx는 Quantum으로 가는 모든 요청마다 (`/public/` 공유 링크 경로 제외) `auth_request /_auth`로 nas-gateway에 사용자를 묻는다.

```
1. 유효한 세션 쿠키가 있음      → 세션의 사용자       (계정 전환 상태)
2. 클라이언트 IP가 매핑 표에 있음 → 매핑된 사용자       (예: 집 와이파이 → shared)
3. 둘 다 아님                   → 401 → 로그인 페이지로 이동
```

응답은 `X-Nas-User` 헤더로 돌려주고, Nginx가 이를 `X-Username`으로 Quantum에 전달한다.

- 클라이언트 IP는 Nginx가 `$remote_addr`로 설정한 `X-Real-IP`만 사용한다. (게이트웨이는 `127.0.0.1`에서만 수신)
- IP 매핑은 위에서부터 처음 일치하는 항목을 사용한다. IPv4/IPv6 CIDR 모두 지원.

## 계정 구분

| 계정 | 로그인 방식 | Quantum 권한 |
|---|---|---|
| 개인 계정 (예: `owner`) | IP 매핑(본인 네트워크만) 또는 비밀번호 로그인 | 관리자, 공유 가능. 전체 접근 |
| 공용 계정 (예: `shared`) | **IP 매핑으로만** (비밀번호 로그인 불가) | 일반 사용자. scope `/shared` |
| 추가 사용자 | 비밀번호 로그인 | 일반 사용자. scope `/users/<이름>` |

- Quantum은 처음 보는 proxy 사용자를 **자동 생성**한다 (관리자 아님, 공유 권한 없음, 자기 폴더 `/users/<이름>`). 관리자 승격과 공용 계정 scope 지정은 구축 시 한 번 수행한다.
- 사용자 이름 규칙: 영문 소문자, 숫자, `.` `_` `-`, 최대 32자. 폴더 이름으로도 쓰이기 때문. `admin`은 Quantum이 자동으로 관리자로 만드는 이름이라 금지. (게이트웨이가 설정 로드 시 검사)

## 세션

| 항목 | 사양 |
|---|---|
| 세션 ID | 256비트 무작위 값 (`crypto/rand`) |
| 저장소 | 게이트웨이 메모리. 재시작 시 로그인 세션만 소멸 (IP 매핑 사용자는 영향 없음) |
| 쿠키 | `__Host-nas_session`, `Path=/`, `HttpOnly`, `Secure`, `SameSite=Lax` |
| 만료 | 마지막 사용 후 7일, 발급 후 최대 30일 |
| 로그인 시 | 기존 세션 폐기 후 새 ID 발급 (세션 고정 방지) |
| 로그아웃 | 서버 세션 삭제 + 쿠키 삭제 → IP 매핑 계정으로 복귀 |
| 전체 로그아웃 | 해당 사용자의 모든 세션 삭제 |

JWT 방식은 만료 전 즉시 무효화가 불가능해서 채택하지 않는다.

## 엔드포인트

| 경로 | 메서드 | 인증 | 설명 |
|---|---|---|---|
| `/_auth` | GET | - | Nginx 내부 전용. 200 + `X-Nas-User` / 401 / 403 |
| `/_gw/login` | GET | 없음 | 로그인 페이지 (사용자 디자인 HTML) |
| `/_gw/login` | POST | 없음 | `username`, `password` 검증 → 세션 발급 → `/`로 이동 |
| `/_gw/logout` | POST | 세션 | 현재 세션 삭제 |
| `/_gw/logout-all` | POST | 세션 | 사용자의 모든 세션 삭제 |
| `/_gw/whoami` | GET | 세션 또는 IP | 현재 사용자, 판단 근거(session/ip), 연결 상태(bound) 반환 |
| `/_gw/reset` | GET | 세션 또는 IP | Quantum 쿠키 만료 + 연결 쿠키를 현재 사용자로 기록 후 `/`로 이동. 사용자가 없으면 로그인 페이지로 |
| `/_gw/vault` | GET | 세션 또는 IP | 암호화 페이지 |
| `/_gw/static/*` | GET | 없음 | 정적 파일 (overlay.js, CSS, age 라이브러리 등) |

- 상태를 바꾸는 요청(POST)은 `Origin` 헤더가 자기 도메인일 때만 허용한다.
- 로그인 후 이동 위치는 항상 `/` (오픈 리다이렉트 방지).

## Quantum 화면의 계정 메뉴

Quantum 화면에는 계정 전환 버튼을 넣을 자리가 없고 사용자 JS를 넣는 설정도 없으므로, Nginx `sub_filter`로 Quantum HTML의 `</head>` 앞에 `/_gw/overlay.js`를 주입한다. (Quantum 소스 수정 없음. Quantum의 CSP는 같은 출처 외부 스크립트를 허용함)

overlay.js 기능:
- 현재 사용자 표시 (IP 매핑 상태인지 로그인 상태인지 포함)
- 계정 전환(로그인 페이지 이동), 로그아웃, 암호화 보관함 링크
- 삭제 확인 창에 "휴지통 없이 바로 영구 삭제됩니다" 안내 추가
- 연결 상태 확인: 페이지 로드 시와 창에 다시 포커스될 때 `/_gw/whoami`를 확인해서, 연결이 어긋났으면 `/_gw/reset`, 사용자가 없으면 `/_gw/login`으로 이동

## Quantum 세션 쿠키 문제

Quantum은 proxy 모드에서도 자체 세션 쿠키(`filebrowser_quantum_jwt`, HttpOnly)를 발급하고, **유효한 쿠키가 있으면 `X-Username` 헤더보다 쿠키를 우선**한다. 토큰에는 사용자 이름도 들어 있지 않다. 그래서 계정 전환이나 로그아웃 후 이전 계정의 쿠키가 남아 있으면 이전 계정으로 동작한다.

해결: **연결 사용자 쿠키** (`__Host-nas_bound`)

- 게이트웨이가 "이 브라우저는 지금 어떤 사용자로 Quantum에 연결되어 있는가"를 이 쿠키에 기록한다.
- `/_auth`는 판단된 사용자와 연결 쿠키가 다르거나 연결 쿠키가 없으면 **403** → Nginx가 `/_gw/reset`으로 보낸다.
- `/_gw/reset`은 Quantum 쿠키를 **서버 응답으로 만료**시키고(HttpOnly라 스크립트로는 못 지움), 연결 쿠키를 현재 사용자로 기록한 뒤 `/`로 보낸다. 다음 요청부터 Quantum은 쿠키가 없으므로 헤더 기준으로 새 세션을 만든다.
- 로그인, 로그아웃, 전체 로그아웃은 모두 `/_gw/reset`을 거친다.
- 연결 쿠키를 조작해도 결과는 reset이 다시 일어나는 것뿐이라 서명하지 않는다.

추가 방어 (Nginx):
- Quantum은 `?auth=` 쿼리와 `Authorization` 헤더로도 토큰을 받는다. 게이트웨이가 모르는 토큰으로 다른 사용자처럼 동작하지 못하도록, Nginx가 `Authorization` 헤더를 지우고 `auth` 쿼리가 있는 요청은 거부한다. 브라우저는 쿠키로만 인증한다.
- 사용자 기본 권한에서 API 토큰 발급을 끈다.

## 무차별 대입 방어

- Nginx `limit_req`: `/_gw/login` POST에 IP당 분당 10회
- 게이트웨이: 사용자별 연속 5회 실패 시 15분 잠금 (잠금 중에도 응답 메시지는 동일하게 유지)
- 로그인 성공/실패를 IP와 함께 로그에 기록

## 설정 파일 (`gateway/config.yaml`)

```yaml
listen: 127.0.0.1:8086
public_origin: https://nas.example.com

session:
  idle_timeout: 168h
  absolute_timeout: 720h

ip_map:                       # 위에서부터 첫 일치 사용
  - cidr: 192.168.0.0/24      # 집 와이파이 (실제 찍히는 IP는 구축 시 확인)
    user: shared
  - cidr: 203.0.113.10/32     # 본인만 쓰는 고정 IP (공유 네트워크는 매핑 금지, 05-review B3)
    user: owner

users:
  owner:
    password_hash: "$2a$12$..."   # nas-gateway hash-password 로 생성
  shared:
    login: false                  # IP 매핑 전용
```

- `SIGHUP`으로 설정 재적용 (세션 유지). 사용자 삭제·비밀번호 변경 시 해당 사용자 세션은 폐기.
- 비밀번호 해시 생성: `nas-gateway hash-password`
