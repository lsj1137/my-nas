#!/usr/bin/env bash
# 로컬 개발 환경 준비. 운영 설정(nginx/, filebrowser/config.example.yaml)을 그대로 가져와
# 로컬용 값(nas.localhost:8443, 컨테이너 주소)만 바꿔서 dev/.generated/ 에 만든다.
# 운영 설정을 고친 뒤에는 이 스크립트를 다시 실행하면 개발 환경에도 반영된다.
set -euo pipefail
cd "$(dirname "$0")"

DOMAIN=nas.localhost
PORT=8443
OUT=.generated

mkdir -p "$OUT"/{certs,nginx,srv/shared,srv/users,fbdata}

# 1) 자체 서명 인증서 (브라우저 경고는 처음 한 번 "고급 → 계속 진행")
if [ ! -f "$OUT/certs/fullchain.pem" ]; then
  MSYS_NO_PATHCONV=1 openssl req -x509 -newkey rsa:2048 -nodes -days 825 \
    -subj "/CN=$DOMAIN" -addext "subjectAltName=DNS:$DOMAIN" \
    -keyout "$OUT/certs/privkey.pem" -out "$OUT/certs/fullchain.pem" 2>/dev/null
fi

# 2) Nginx: 운영 설정에서 도메인과 업스트림 주소만 바꾼다
cp ../nginx/nas-http.conf "$OUT/nginx/"
sed -e "s#nas.example.com#$DOMAIN#g" \
    -e "s#127.0.0.1:8085#filebrowser:80#g" \
    -e "s#127.0.0.1:8086#nas-gateway:8086#g" \
    ../nginx/nas.conf > "$OUT/nginx/nas.conf"
# 컨테이너 안은 443이지만 브라우저는 8443으로 접속하므로, 리다이렉트를 상대 경로로 보낸다
echo "absolute_redirect off;" > "$OUT/nginx/dev-redirect.conf"

# 3) Quantum 설정: 운영 예시 그대로
cp ../filebrowser/config.example.yaml "$OUT/fbdata/config.yaml"

# 4) 게이트웨이 설정: 덮어쓰지 않음 (IP 매핑을 바꿔 가며 테스트할 수 있게)
if [ ! -f gateway.yaml ]; then
  cp gateway.example.yaml gateway.yaml
fi

echo "준비 완료. 실행: docker compose -f dev/docker-compose.yml up -d --build"
echo "접속: https://$DOMAIN:$PORT   (로그인: owner / dev-password)"
