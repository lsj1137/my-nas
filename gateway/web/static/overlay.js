// Quantum 화면에 Nginx sub_filter로 주입되는 스크립트.
// 1) 계정 메뉴 (현재 사용자, 계정 전환, 로그아웃, 암호화 보관함)
// 2) 게이트웨이와 Quantum 세션 연결 상태 확인
// 3) 삭제 확인 창에 영구 삭제 안내 추가
(() => {
  "use strict";
  if (window.__nasOverlay || location.pathname.startsWith("/public/")) return;
  window.__nasOverlay = true;

  const css = document.createElement("link");
  css.rel = "stylesheet";
  css.href = "/_gw/static/overlay.css";
  document.head.append(css);

  // ---- 연결 상태 확인 ----
  let me = null;

  async function checkSession() {
    let res;
    try {
      res = await fetch("/_gw/whoami", { credentials: "same-origin", cache: "no-store" });
    } catch {
      return; // 네트워크 오류는 무시하고 다음 확인 때 다시 시도
    }
    if (res.status === 401) {
      location.href = "/_gw/login";
      return;
    }
    if (!res.ok) return;
    const next = await res.json();
    if (!next.bound) {
      location.href = "/_gw/reset";
      return;
    }
    me = next;
    renderMenu();
  }

  document.addEventListener("visibilitychange", () => {
    if (document.visibilityState === "visible") checkSession();
  });
  setInterval(checkSession, 5 * 60 * 1000);

  // ---- 계정 메뉴 ----
  let root = null;

  function el(tag, attrs = {}, ...children) {
    const e = document.createElement(tag);
    for (const [k, v] of Object.entries(attrs)) {
      if (k === "onclick") e.addEventListener("click", v);
      else e.setAttribute(k, v);
    }
    for (const c of children) e.append(c);
    return e;
  }

  function postTo(action) {
    const form = el("form", { method: "post", action });
    document.body.append(form);
    form.submit();
  }

  function renderMenu() {
    if (!me || !document.body) return;
    if (!root) {
      root = el("div", { id: "nas-overlay" });
      document.body.append(root);
      document.addEventListener("click", (e) => {
        if (root && !root.contains(e.target)) root.classList.remove("open");
      });
    }
    const viaText = me.via === "session" ? "로그인" : "자동";
    const avatar = () => el("span", { class: "nas-avatar", "aria-hidden": "true" }, me.user.slice(0, 1).toUpperCase());
    const item = (tag, attrs, icon, label) => el(tag, { ...attrs, class: `nas-item nas-icon-${icon}` }, label);

    const items = [
      el("div", { class: "nas-who" }, avatar(), el("div", {}, el("strong", {}, me.user), el("span", { class: `nas-badge ${me.via}` }, viaText))),
      el("div", { class: "nas-sep" }),
      item("a", { href: "/_gw/vault" }, "lock", "보관함"),
      item("a", { href: "/_gw/login" }, "switch", "계정 전환"),
    ];
    if (me.via === "session") {
      items.push(el("div", { class: "nas-sep" }));
      items.push(item("button", { type: "button", onclick: () => postTo("/_gw/logout") }, "logout", "로그아웃"));
      items.push(item("button", { type: "button", onclick: () => postTo("/_gw/logout-all") }, "devices", "전체 로그아웃"));
    }
    root.replaceChildren(
      el("button", { type: "button", class: "nas-toggle", "aria-label": "계정 메뉴", "aria-haspopup": "menu", onclick: () => root.classList.toggle("open") },
        avatar(), el("span", { class: "nas-name" }, me.user)),
      el("div", { class: "nas-menu", role: "menu" }, ...items),
    );
    syncTheme();
  }

  // ---- Quantum 테마 따라가기 ----
  // Quantum은 다크 테마일 때 #main 등에 dark-mode 클래스를 붙인다.
  function syncTheme() {
    if (!root) return;
    const main = document.getElementById("main");
    const dark = main ? main.classList.contains("dark-mode") : null;
    root.classList.toggle("nas-dark", dark === true);
    root.classList.toggle("nas-light", dark === false);
  }

  new MutationObserver(syncTheme).observe(document.documentElement, { subtree: true, attributes: true, attributeFilter: ["class"] });

  // ---- 삭제 확인 창 안내 ----
  const DELETE_DIALOG = '[aria-label="delete-prompt"]';
  const WARNING_TEXT = "영구 삭제 · 복구 불가";

  function decorateDeleteDialogs() {
    for (const dialog of document.querySelectorAll(DELETE_DIALOG)) {
      if (dialog.querySelector(".nas-delete-warning")) continue;
      const warning = el("p", { class: "nas-delete-warning" }, WARNING_TEXT);
      const content = dialog.querySelector(".card-content") || dialog;
      content.append(warning);
    }
  }

  new MutationObserver(decorateDeleteDialogs).observe(document.documentElement, { childList: true, subtree: true });

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", checkSession);
  } else {
    checkSession();
  }
})();
