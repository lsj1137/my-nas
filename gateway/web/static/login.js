"use strict";

if (new URLSearchParams(location.search).has("error")) {
  document.getElementById("gw-error").hidden = false;
}

// IP 매핑으로 이미 접속 가능한 상태면 "돌아가기" 링크를 보여준다 (계정 전환하러 온 경우).
fetch("/_gw/whoami", { credentials: "same-origin" })
  .then((r) => (r.ok ? r.json() : null))
  .then((me) => {
    if (me && me.user) document.getElementById("gw-back").hidden = false;
  })
  .catch(() => {});
