# 01. 요구사항과 전체 구조

## 요구사항

| # | 요구사항 |
|---|---|
| R1 | 어디서든 브라우저로 주소만 입력해 파일 업로드/다운로드 (VPN·전용 앱 없이) |
| R2 | 새 도메인을 구매하지 않고 기존 도메인의 서브도메인 사용 (`nas.<기존도메인>`) |
| R3 | 허용 IP별로 계정을 매핑해 로그인 없이 접속 (예: 집 와이파이 → 공용 계정) |
| R4 | 허용 IP에서도 비밀번호를 입력해 개인 계정으로 전환 가능, 로그아웃 시 IP 매핑 계정으로 복귀 |
| R5 | 매핑되지 않은 IP는 로그인 필요 |
| R6 | 로그인 화면은 직접 디자인한 페이지 사용 (브라우저 Basic Auth 팝업 사용 안 함) |
| R7 | 사용자별로 볼 수 있는 폴더 분리 |
| R8 | 서버가 해킹당해도 내용을 볼 수 없는 파일 암호화 (클라이언트 측 암호화). 암호화 파일은 미리보기·검색 없이 파일명만 표시. 파일명 노출은 허용 |
| R9 | 공유 링크: 링크로 들어온 사람은 공유된 파일만 보고 다운로드할 수 있음 (다른 파일은 볼 수 없음) |
| R10 | 삭제는 휴지통 없이 즉시 삭제. 삭제 확인 창에 "휴지통 없이 바로 영구 삭제됩니다" 안내 표시 |
| R11 | 개인(관리자) 계정의 IP 매핑은 본인만 쓰는 네트워크에만 설정 |

## 전체 구조

```
[브라우저]
   │ https://nas.<기존도메인>
   ▼
[공유기 443] ─▶ [기존 Nginx]
                  │  ├─ HTTPS 종료 (Let's Encrypt, certbot)
                  │  └─ 모든 요청마다 auth_request ─▶ [nas-gateway  127.0.0.1:8086]
                  │                                      ├─ 인증 판단: 세션 쿠키 → IP 매핑 → 거부
                  │                                      ├─ /_gw/login, /_gw/logout  (계정 전환)
                  │                                      ├─ /_gw/vault               (브라우저 암호화 페이지)
                  │                                      └─ /_gw/static/overlay.js   (Quantum 화면에 붙는 계정 메뉴)    
                  │ X-Username: <판단된 계정>
                  │
                  │ (/public/ 공유 링크 경로만 인증 없이 통과, X-Username은 빈 값으로 덮어씀)
                  ▼
           [FileBrowser Quantum  127.0.0.1:8085, proxy 인증]
                  ▼
              /srv/nas  (일반 파일 + .age 암호화 파일)
```

## 파일 관리자 선택: FileBrowser Quantum

원본 File Browser(`filebrowser/filebrowser`)는 2026-09-01 아카이브되어 보안 수정이 중단되었다. 그래서 원본에서 갈라져 나와 활발히 개발 중인 **FileBrowser Quantum**(`gtsteffaniak/filebrowser`, 이미지 `gtstef/filebrowser`)을 사용한다.

- **stable 채널 버전 고정.** 베타는 사용하지 않는다.
- 개인 1명이 주도하는 프로젝트라 멈출 위험이 있다. 게이트웨이와 Quantum 사이는 `X-Username` 헤더 하나로만 연결해서, 나중에 다른 파일 관리자로 교체하기 쉽게 둔다.
- 보안 권고(GitHub Security Advisories)를 주기적으로 확인하고 계획적으로 업데이트한다.

## 기술 결정

| 항목 | 결정 | 이유 |
|---|---|---|
| 리버스 프록시 | 기존 Nginx | 80/443 이미 사용 중, 관리 방식 통일 |
| HTTPS 인증서 | certbot `certonly --nginx` | 인증서만 발급하고 Nginx 설정은 직접 관리 |
| 웹 파일 관리자 | FileBrowser Quantum stable (Docker, 버전 고정) | 보안 수정이 계속됨. 파일을 원본 그대로 다룸. 소스 수정 없음 |
| 파일 관리자 인증 | proxy 모드 (`X-Username` 헤더 신뢰). 비밀번호 로그인·회원가입 끔 | 인증은 nas-gateway가 전담 |
| 인증 게이트웨이 | nas-gateway 자체 개발 (Go) | IP 매핑 + 계정 전환 + 커스텀 로그인 페이지는 기존 도구로 불가 |
| 인증 방식 | 서버 세션 (세션 ID를 쿠키에 저장) | 서버 1대 구성, 즉시 로그아웃·차단 필요 |
| 암호화 | 브라우저 내 age 형식 암호화 (passphrase) | 서버에 키가 가지 않음. 표준 형식이라 `age` CLI로도 복호화 가능 |
| 실행 방식 | Docker Compose (Quantum, nas-gateway) | 기존 서비스와 분리 |
| 저장 위치 | `/srv/nas` | 디스크 추가 시 이 경로에 마운트 |
| DNS | `nas` A 레코드 → 집 공인 IP (Cloudflare 사용 시 DNS only) | 정확한 클라이언트 IP 판단, 업로드 용량 제한 회피 |

## 포트

| 포트 | 대상 | 노출 |
|---|---|---|
| 443, 80 | Nginx | 외부 |
| 8085 | FileBrowser Quantum | `127.0.0.1` 전용 |
| 8086 | nas-gateway | `127.0.0.1` 전용 |

Quantum과 nas-gateway는 `X-Username`, `X-Real-IP` 헤더를 신뢰하므로 **절대 외부에 노출하면 안 된다.**

## 경로 규칙

| 경로 | 처리 |
|---|---|
| `/_gw/*` | nas-gateway (로그인 페이지 등 일부는 인증 없이 접근) |
| `/_auth` | Nginx 내부 전용 (외부 요청 불가) |
| `/public/` | 공유 링크. 인증 없이 Quantum으로 전달 (`X-Username`은 빈 값으로 덮어씀) |
| 그 외 전체 | 인증 후 Quantum |

Quantum이 쓰는 경로(`/api`, `/public` 등)와 겹치지 않도록 게이트웨이는 `/_gw/` 접두사만 사용한다.
