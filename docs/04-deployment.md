# 04. 서버 구축 절차

`nas.example.com`은 실제 도메인으로, `owner`는 실제 개인 계정 이름으로 바꿔서 사용한다.

## 저장소 구성

```
nas/
 ├─ docker-compose.yml           # Quantum + nas-gateway
 ├─ .env.example                 # → .env (서명 키, NAS 경로)          [git 제외]
 ├─ filebrowser/
 │   ├─ config.example.yaml      # → config.yaml (Quantum 설정)         [git 제외]
 │   └─ data/                    # Quantum DB, 캐시                     [git 제외]
 ├─ gateway/
 │   ├─ config.example.yaml      # → config.yaml (사용자, IP 매핑)       [git 제외]
 │   ├─ custom/                  # 직접 디자인한 login.html 등 (선택)     [git 제외]
 │   └─ (Go 소스, Dockerfile, web/)
 ├─ nginx/
 │   ├─ nas-http.conf            # → /etc/nginx/conf.d/
 │   └─ nas.conf                 # → /etc/nginx/sites-available/nas
 └─ docs/
```

## 1. 사전 확인

```bash
docker --version && docker compose version   # 없으면: curl -fsSL https://get.docker.com | sh
nginx -v
sudo ss -tlnp | grep -E ':8085|:8086'        # 비어 있어야 함
```

## 2. 저장소 받기

```bash
git clone <원격 저장소 주소> ~/nas
cd ~/nas
```

## 3. 저장 공간

Quantum 컨테이너는 UID 1000으로 실행되므로 폴더 소유자를 1000으로 맞춘다.

```bash
sudo mkdir -p /srv/nas/{shared,users}
sudo chown -R 1000:1000 /srv/nas
mkdir -p filebrowser/data && sudo chown 1000:1000 filebrowser/data
```

## 4. 설정 파일 만들기

```bash
cp .env.example .env && chmod 600 .env
cp filebrowser/config.example.yaml filebrowser/config.yaml
cp gateway/config.example.yaml gateway/config.yaml
```

| 파일 | 수정할 것 |
|---|---|
| `.env` | `FILEBROWSER_JWT_TOKEN_SECRET`에 `openssl rand -hex 48` 결과 |
| `filebrowser/config.yaml` | `adminUsername`(개인 계정 이름). 도메인은 쓰지 않는다 |
| `gateway/config.yaml` | `public_origin`, `ip_map`, `users` |

`filebrowser/config.yaml`은 **반드시 컨테이너를 띄우기 전에** 만들어야 한다. 없으면 Docker가 같은 이름의 빈 폴더를 만들어 버린다.

게이트웨이 비밀번호 해시 생성:
```bash
docker compose build nas-gateway
docker compose run --rm -it --no-deps nas-gateway hash-password
```
출력된 `$2a$12$...` 값을 `gateway/config.yaml`의 `password_hash`에 넣는다.

직접 디자인한 로그인 페이지가 있으면 `gateway/custom/login.html`에 둔다. (규칙은 `gateway/web/login.html` 상단 주석 참고)

## 5. 실행

```bash
docker compose up -d --build
docker compose ps
docker compose logs filebrowser | tail -20    # 설정 오류가 있으면 여기 나옴
curl -s -o /dev/null -w '%{http_code}\n' http://127.0.0.1:8085/health   # 200
```

Quantum은 첫 실행 때만 임의의 서명 키를 쓰고, `.env`의 키는 재시작 후부터 적용된다. 첫 실행 직후 한 번 재시작한다.
```bash
docker compose restart filebrowser
```

## 6. DNS

`nas` A 레코드 → 집 공인 IP. Cloudflare 사용 시 DNS only(회색 구름).
확인: `nslookup nas.example.com`

## 7. 인증서

```bash
sudo certbot certonly --nginx -d nas.example.com
```

## 8. Nginx

```bash
sudo cp nginx/nas-http.conf /etc/nginx/conf.d/nas-http.conf
sudo cp nginx/nas.conf /etc/nginx/sites-available/nas
sudo sed -i 's/nas.example.com/<실제 도메인>/g' /etc/nginx/sites-available/nas
sudo ln -s /etc/nginx/sites-available/nas /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
```

`nginx -t`가 실패하면 reload가 실행되지 않으므로 기존 서비스에는 영향이 없다.

## 9. Quantum 초기 설정 (개인 계정으로)

1. 개인 계정이 매핑된 네트워크에서 접속하거나, 로그인 페이지에서 개인 계정으로 로그인한다.
   - `adminUsername`과 같은 이름이므로 자동으로 관리자가 된다.
2. 집 와이파이에서 한 번 접속해 `shared` 사용자가 자동 생성되게 한다.
3. 개인 계정으로 돌아와 **설정 → 사용자 관리**에서:
   - `shared`: Scopes를 `/shared`로 변경
   - `owner`: 공유(share) 권한 켜기, Scope를 `/`(전체)로 변경
4. 프로필 설정의 "확인 메시지 없이 파일 삭제"가 꺼져 있는지 확인한다.

## 10. 검증

| 테스트 | 기대 결과 |
|---|---|
| Nginx 접속 로그에서 집 와이파이 접속 시 찍히는 IP 확인 | 그 값을 `ip_map`에 반영 후 `docker compose kill -s HUP nas-gateway` |
| 집 와이파이로 접속 | 로그인 없이 `shared`로 `/shared`만 보임. 왼쪽 아래 계정 메뉴 표시 |
| 계정 메뉴 → 다른 계정으로 전환 → owner 로그인 | owner 화면으로 전환 |
| 계정 메뉴 → 로그아웃 | 다시 `shared` 화면 |
| 휴대폰 데이터로 접속 | 로그인 페이지 |
| 틀린 비밀번호 6회 | 계속 실패, `docker compose logs nas-gateway`에 기록 |
| `https://nas.example.com/_auth` 직접 접속 | 404 |
| 서버 외부에서 8085, 8086 포트 접속 | 연결 불가 |
| `https://nas.example.com/?auth=x` | Quantum에 도달하지 않고 `/_gw/reset`으로 이동 |
| 파일 삭제 | 확인 창에 영구 삭제 안내 표시 |
| owner로 파일 공유 링크 생성 → 휴대폰 데이터로 열기 | 로그인 없이 그 파일만 보이고 다운로드됨 |
| 1GB 파일 업로드/다운로드 | 정상 |
| 암호화 보관함: 암호화 업로드 → 복호화 다운로드 | 원본과 동일 (`sha256sum` 비교) |
| `.age` 파일을 PC에서 `age -d`로 복호화 | 원본과 동일 |

## 11. 운영

| 작업 | 방법 |
|---|---|
| IP 매핑·사용자 변경 | `gateway/config.yaml` 수정 → `docker compose kill -s HUP nas-gateway` (세션 유지) |
| 게이트웨이 코드 업데이트 | `git pull && docker compose up -d --build nas-gateway` |
| Quantum 업데이트 | 보안 권고 확인 → `docker-compose.yml`의 태그와 digest 변경 → `docker compose up -d filebrowser` |
| 보안 권고 확인 | https://github.com/gtsteffaniak/filebrowser/security/advisories |
| 백업 대상 | `/srv/nas`(파일), `filebrowser/data`(사용자, 공유 링크), `gateway/config.yaml`, `.env` |
