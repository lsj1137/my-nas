# 05. 설계 검토 결과

## A. 검토 중 발견해 설계에 반영한 구멍

| # | 구멍 | 반영 내용 | 문서 |
|---|---|---|---|
| A1 | 계정 전환 버튼을 둘 곳이 Quantum 화면에 없음 | Nginx `sub_filter`로 overlay.js 주입 (소스 수정 없음) | 02 |
| A2 | Quantum은 유효한 자체 쿠키가 있으면 `X-Username`보다 쿠키를 우선 → 전환/로그아웃 후 이전 계정으로 동작 | 연결 사용자 쿠키(`__Host-nas_bound`) + 불일치 시 403 → `/_gw/reset`이 Quantum 쿠키를 서버에서 만료 | 02 |
| A3 | `X-Real-IP`, `X-Username` 헤더 위조 | 게이트웨이·Quantum 모두 `127.0.0.1` 전용, IP는 Nginx `$remote_addr`만 사용 | 01, 02 |
| A4 | 로그인 무차별 대입 | Nginx limit_req + 사용자별 잠금 + 로그 기록 | 02 |
| A5 | 로그인 후 리다이렉트 조작 (오픈 리다이렉트) | 로그인 후 항상 `/`로 이동 | 02 |
| A6 | 공용 계정 비밀번호가 유출되면 외부에서 로그인 가능 | 공용 계정은 IP 매핑 전용, 비밀번호 로그인 불가 | 02 |
| A7 | 업로드 크기 제한(Nginx 기본 1MB), 대용량 타임아웃 | `client_max_body_size 0`, 버퍼링 끔, 타임아웃 1시간 | 04 |
| A8 | 게이트웨이 경로가 Quantum 경로와 충돌 | 게이트웨이는 `/_gw/` 접두사만 사용 | 01 |
| A9 | 암호화 페이지에 외부 스크립트가 끼어들 위험 | 라이브러리 자체 제공(체크섬 기록), 엄격한 CSP | 03 |
| A10 | IP 매핑 변경 시 재시작하면 세션이 사라짐 | `SIGHUP`으로 설정 재적용 | 02 |
| A11 | POST 요청 위조 (CSRF) | `SameSite=Lax` + `Origin` 검사 | 02 |
| A12 | 원본 File Browser 아카이브 (보안 수정 중단) | FileBrowser Quantum stable로 교체, 태그+digest 고정 | 01 |
| A13 | Quantum은 `?auth=`, `Authorization`으로도 토큰을 받음 → 게이트웨이가 모르는 신원으로 동작 가능 | Nginx가 `Authorization` 제거, `auth` 쿼리 요청 거부. 사용자 API 권한 끔 | 02, nginx |
| A14 | Quantum은 이름이 `adminUsername`(기본 `admin`)인 proxy 사용자를 자동으로 관리자로 만듦 | `adminUsername`을 개인 계정 이름으로 지정, 게이트웨이는 `admin` 이름 금지 | 02, 04 |
| A15 | 사용자 이름의 `/` 등이 Quantum 폴더 경로에 그대로 들어감 | 게이트웨이가 이름 형식 검사 (`^[a-z0-9][a-z0-9._-]{0,31}$`) | 02 |
| A16 | Quantum 서명 키를 비우면 재시작 후 빈 키로 서명 → 토큰 위조 (GHSA-8f9r) | `.env`의 `FILEBROWSER_JWT_TOKEN_SECRET` 필수 (없으면 compose 실행 거부) | 04 |
| A17 | Quantum은 `X-Forwarded-Host`, `X-Forwarded-Proto`를 그대로 믿음 (쿠키 Domain/Secure, 공유 URL) | Nginx가 모든 프록시 location에서 덮어씀 | nginx |
| A18 | Quantum 비밀번호 로그인은 기본값이 켜짐 | 설정에서 명시적으로 끔 | filebrowser/config.example.yaml |
| A20 | 로그인 시도 제한이 로그인 **페이지 열기(GET)**까지 세어서 새로고침 몇 번에 막힘 | POST만 세도록 변경 (`map $request_method`) | nginx/nas-http.conf |
| A21 | Quantum 내장 로그인 제한(IP당 분당 10회)이 화면을 열 때마다 호출되는 `/api/auth/login`에 걸림. 집 안 기기는 모두 공유기 IP 하나로 보여 여럿이 쓰면 화면이 깨짐 | `http.disableRateLimit: true`. Quantum 비밀번호 로그인은 꺼져 있고 실제 로그인 제한은 Nginx + 게이트웨이가 담당 | filebrowser/config.example.yaml |
| A22 | 예시 설정의 도메인(`nas.example.com`)이 서버 설정에 남아 로그아웃이 엉뚱한 주소로 이동 | Quantum 설정에서 도메인 제거 (`externalUrl` 없음, `logoutRedirectUrl: /_gw/logout`) | filebrowser/config.example.yaml |
| A19 | `Referrer-Policy: no-referrer`면 브라우저가 폼 POST의 Origin을 `null`로 보내 정상 로그인이 Origin 검사에 막힘 (브라우저 테스트에서 발견) | `same-origin`으로 변경, 회귀 테스트 추가 | gateway/server.go |

## B. 결정 사항

| # | 항목 | 결정 |
|---|---|---|
| B1 | 공유 링크 | **사용.** `/public/` 경로만 인증 예외. 링크로 들어온 사람은 공유된 파일만 보고 다운로드 가능. 공유 권한은 개인 계정에만 부여 |
| B2 | 삭제 | 휴지통 없이 즉시 삭제. overlay.js가 삭제 확인 창에 영구 삭제 안내를 추가 |
| B2-1 | 백업 | **미정.** restic 사용, 대상 위치는 나중에 결정 |
| B3 | 개인 계정 IP 매핑 | 본인만 쓰는 네트워크에만 매핑 |
| B4 | 배포 | 이 저장소를 git 원격 저장소에 올리고 서버에서 clone |
| B5 | 서버 전용 값 (도메인, 계정, IP, 비밀 키) | `.env`, `gateway/config.yaml`, `filebrowser/config.yaml`에만 입력. 저장소에는 `*.example*`만 |
| B6 | 파일 관리자 | FileBrowser Quantum `1.5.8-stable` (digest 고정) |

## C. 확인 결과

| # | 확인 항목 | 결과 |
|---|---|---|
| C1 | Quantum 세션 방식 | 쿠키 `filebrowser_quantum_jwt` (HttpOnly, SameSite=Strict, Domain=요청 호스트). 토큰에 사용자 이름 없음. 추출 순서: 쿠키 → `?auth=` → `Authorization` → A2, A13 반영 |
| C2 | 주입 스크립트 CSP | index의 CSP는 `script-src 'self' 'nonce-…' …` → 같은 출처 외부 스크립트 허용 |
| C3 | 공유 링크 경로 | `/public/` 하나로 페이지·정적 파일·API 모두 포함. 익명 방문자는 로그인 API를 호출하지 않음 |
| C4 | 파일 API | 목록 `GET /api/resources?source=&path=`, 업로드 `POST /api/resources?source=&path=&override=` (raw body, 10MB 분할은 `X-File-Chunk-Offset`/`X-File-Total-Size`), 다운로드 `GET /api/resources/download?source=&file=` |
| C5 | 삭제 확인 창 선택자 | `[aria-label="delete-prompt"]` (언어와 무관하게 고정) |
| C6 | age JS 라이브러리 | v0.3.1 (BSD-3), 스트리밍 지원. 9MB 왕복 일치, **공식 Go age 구현으로 복호화 성공** (CLI 호환 확인) |
| C7 | 집 와이파이 접속 시 Nginx에 찍히는 IP | **구축 시 확인** (04 검증 항목) |
| C8 | 통합 테스트 (2026-10-05, 로컬 Docker: Nginx + 게이트웨이 + Quantum 1.5.8) | **통과.** 아래 F절 참고 |

## D. 감수하기로 한 한계

| # | 한계 | 이유 |
|---|---|---|
| D1 | 파일명은 암호화되지 않음 | 사용자 결정 |
| D2 | 서버가 장악된 이후에 입력한 암호화 비밀번호는 탈취될 수 있음 | 웹 기반 암호화의 근본적 한계. 중요 파일은 age CLI 병행 |
| D3 | 게이트웨이 재시작 시 로그인 세션 소멸 | 단순성. IP 매핑 사용자는 영향 없음 |
| D4 | 집 와이파이에 접속한 손님도 공용 계정으로 접속됨 | 공용 계정은 `/shared`만 접근 가능 |
| D5 | 집 공인 IP가 바뀌면 공인 IP 기준 매핑이 풀림 | 풀리면 로그인 페이지가 나오는 안전한 방향으로 실패 |
| D6 | 사용 중 세션이 만료되거나 연결이 어긋나면 화면에서 오류가 날 수 있음 | overlay.js가 창 포커스 시와 5분마다 확인해서 자동으로 다시 연결 |
| D7 | 암호화 비밀번호 분실 시 복구 불가 | 클라이언트 측 암호화의 본질 |
| D8 | 사용자가 프로필에서 "확인 메시지 없이 파일 삭제"를 켜면 삭제 안내 창이 뜨지 않음 | Quantum이 서버에서 이 설정을 잠그는 기능이 없음. 기본값은 꺼짐 |
| D9 | GHSA-p7x3 (medium): 공개 공유의 자막/가사 경로가 다운로드 제한을 무시. 1.5.8에서 미해결로 보임 | 이 구성은 다운로드 제한을 쓰지 않으므로 영향 적음. 수정판이 stable에 나오면 업데이트 |
| D10 | Quantum은 개인 1명이 주도하는 프로젝트 | 게이트웨이와 헤더 하나로만 연결해서 교체 가능하게 둠 |
| D11 | 대용량 암호화 파일은 브라우저 메모리(Blob)에 모은 뒤 업로드 | 수 GB 단위는 브라우저에 따라 실패할 수 있음. 향후 스트리밍 업로드로 개선 |

## F. 통합 테스트 결과 (로컬)

curl로 요청 흐름을, 헤드리스 Chrome으로 화면 동작을 확인했다.

| 구분 | 항목 | 결과 |
|---|---|---|
| 인증 | 매핑 IP 첫 접속 → reset → 연결 → Quantum이 `shared`로 인식 | ✅ |
| 인증 | Quantum 쿠키가 남은 상태에서 owner 로그인 → Quantum이 owner(관리자)로 인식 | ✅ |
| 인증 | 로그아웃 → Quantum이 다시 `shared`로 인식 | ✅ |
| 인증 | 틀린 비밀번호, 다른 출처 Origin 로그인 거부 | ✅ |
| 보안 | 연결 쿠키 위조, `X-Username` 위조, `?auth=` → Quantum에 도달하지 못함 (reset으로 돌려보냄) | ✅ |
| 보안 | `/_auth`, `/dav/`, `/swagger` → 404. Quantum/게이트웨이 포트 외부 접근 불가 | ✅ |
| 파일 | 업로드, 10MB 분할 업로드(25MB), 다운로드 원본 일치, 같은 이름 409 | ✅ |
| 권한 | shared는 `/shared` 밖 접근 불가, 공유 링크 생성 불가(403) | ✅ |
| 공유 | 외부인이 공유 링크로 해당 파일만 다운로드, 경로 조작·가짜 해시 차단, 공유 페이지에 계정 메뉴 미주입 | ✅ |
| 운영 | Quantum 재시작 후 세션·공유 링크 유지, 게이트웨이 SIGHUP 후 세션 유지 | ✅ |
| 화면 | 계정 메뉴 표시, 메뉴로 계정 전환·로그아웃 | ✅ |
| 화면 | 삭제 확인 창에 영구 삭제 경고 표시 | ✅ |
| 화면 | 암호화 보관함: 암호화 업로드 → 서버에는 암호문만 저장 → 틀린 비밀번호 거부 → 복호화 결과 원본 일치 | ✅ |
| 화면 | 브라우저 콘솔 오류(CSP 위반 등) 없음 | ✅ |

참고: 금지된 경로 접근 시 Quantum이 404 대신 500을 돌려주는 경우가 있으나 내용은 노출되지 않는다.

## E. 운영 기본값

- Docker 로그 순환: `json-file`, 최대 10MB × 3개
- 컨테이너 재시작 정책: `unless-stopped`
- Quantum 이미지는 태그 + digest 고정 (`latest`, `stable` 금지)
