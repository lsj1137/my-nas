// Quantum 화면의 Material 아이콘을 Lucide 아이콘으로 바꿔 그린다. Nginx가 Quantum HTML에 끼워 넣는다.
// Quantum은 <i class="material-symbols">folder</i> 처럼 "아이콘 이름 글자 + 아이콘 폰트"로 그리므로,
// 이름을 읽어 짝이 있으면 data-lucide 를 붙이고 Lucide SVG를 마스크로 그린다 (quantum.css 참고).
// 짝이 없는 아이콘은 원래 Material 아이콘 그대로 보인다.
(() => {
  "use strict";
  if (window.__nasLucide) return;
  window.__nasLucide = true;

  // Material 이름 → Lucide 이름. 새 아이콘을 추가하면 vendor/lucide/ 에 SVG도 넣어야 한다 (dev/vendor-lucide.cjs).
  const MAP = {
    // 탐색, 기본 동작
    menu: "menu", arrow_back: "arrow-left", arrow_forward: "arrow-right", arrow_upward: "arrow-up", arrow_downward: "arrow-down",
    keyboard_arrow_down: "chevron-down", keyboard_arrow_up: "chevron-up", keyboard_arrow_left: "chevron-left", keyboard_arrow_right: "chevron-right",
    expand_more: "chevron-down", expand_less: "chevron-up", chevron_left: "chevron-left", chevron_right: "chevron-right",
    home: "house", search: "search", search_off: "search-x", manage_search: "file-search", close: "x", check: "check", done: "check",
    add: "plus", remove: "minus", horizontal_rule: "minus", edit: "pencil", delete: "trash-2", delete_forever: "trash-2", delete_sweep: "trash",
    more_vert: "ellipsis-vertical", more_horiz: "ellipsis", swap_vert: "arrow-up-down", swap_horiz: "arrow-left-right", sort: "arrow-down-wide-narrow",
    sort_by_alpha: "arrow-down-a-z", filter_list: "list-filter", tune: "sliders-horizontal", drag_indicator: "grip-vertical", open_in_new: "external-link",
    refresh: "refresh-cw", autorenew: "refresh-cw", replay: "rotate-ccw", restore: "rotate-ccw", undo: "undo-2", redo: "redo-2",
    rotate_right: "rotate-cw", rotate_left: "rotate-ccw", fullscreen: "maximize", fullscreen_exit: "minimize", zoom_in: "zoom-in", zoom_out: "zoom-out",
    select_all: "square-check", check_box: "square-check", check_box_outline_blank: "square", radio_button_checked: "circle-dot",
    // 보기
    view_module: "layout-grid", grid_view: "layout-grid", view_list: "list", list: "list", view_in_ar: "box", visibility: "eye", visibility_off: "eye-off",
    // 파일, 폴더
    folder: "folder", folder_open: "folder-open", create_new_folder: "folder-plus", drive_file_move: "folder-input", folder_shared: "folder-symlink", folder_zip: "file-archive",
    insert_drive_file: "file", description: "file-text", text_snippet: "file-text", note_add: "file-plus", upload_file: "file-up", picture_as_pdf: "file-text",
    file_copy: "copy", content_copy: "copy", content_paste: "clipboard", content_paste_go: "clipboard-paste", archive: "archive",
    file_download: "download", download: "download", download_done: "circle-check", file_upload: "upload", upload: "upload",
    cloud_upload: "cloud-upload", cloud_download: "cloud-download", cloud_off: "cloud-off", sync_problem: "refresh-cw-off",
    image: "image", photo: "image", movie: "film", videocam: "video", audiotrack: "music", music_note: "music", queue_music: "list-music",
    playlist_play: "list-video", lyrics: "mic-vocal", shuffle: "shuffle", play_arrow: "play", pause: "pause", volume_up: "volume-2", code: "code",
    storage: "hard-drive", dns: "server", save: "save", print: "printer", attach_file: "paperclip", link: "link", share: "share-2", qr_code: "qr-code",
    // 계정, 설정
    person: "user", account_circle: "circle-user", person_add: "user-plus", group: "users", groups: "users", group_add: "user-plus",
    settings: "settings", admin_panel_settings: "shield-user", security: "shield", shield: "shield", verified_user: "shield-check",
    login: "log-in", logout: "log-out", exit_to_app: "log-out", key: "key", passkey: "key-round", lock: "lock", lock_reset: "lock-keyhole", lock_open: "lock-open",
    dark_mode: "moon", light_mode: "sun", ads_click: "mouse-pointer-click", push_pin: "pin", language: "languages", palette: "palette",
    build: "wrench", terminal: "terminal", analytics: "chart-column", interests: "shapes", history: "history", schedule: "clock", event: "calendar",
    label: "tag", bookmark: "bookmark", star: "star", favorite: "heart", mail: "mail", send: "send",
    // 상태, 알림
    info: "info", help: "circle-help", warning: "triangle-alert", error: "circle-alert", error_outline: "circle-alert", priority_high: "circle-alert",
    report: "octagon-alert", block: "ban", cancel: "circle-x", highlight_off: "circle-x", check_circle: "circle-check", check_circle_outline: "circle-check",
    add_circle: "circle-plus", remove_circle: "circle-minus", notifications: "bell", notifications_none: "bell", feedback: "message-square-warning",
    sentiment_dissatisfied: "frown", gps_off: "locate-off", computer: "monitor", smartphone: "smartphone", wifi: "wifi",
    // 파일 종류 아이콘 (Quantum utils/mimetype.js 의 materialSymbol)
    file: "file", file_json: "file-json", data_object: "file-braces", css: "file-code", html: "file-code", javascript: "file-code",
    php: "file-code", flutter: "file-code", code_xml: "file-code", terminal_2: "file-terminal", markdown: "file-text", docs: "file-text",
    table: "file-spreadsheet", tsv: "file-spreadsheet", slideshow: "presentation", gif: "file-image", album: "disc-3", menu_book: "book-open",
    package_2: "package", database: "database", memory: "cpu", license: "scroll-text", copyright: "copyright", closed_caption: "captions",
    android: "smartphone", developer_mode_tv: "monitor", architecture: "drafting-compass", blur_circular: "circle-dashed", brush: "brush",
    calendar_month: "calendar", chess_knight: "gamepad-2", game_button_r: "gamepad-2", diamond: "gem", electric_bolt: "zap",
    format_color_text: "type", format_shapes: "shapes", format_underlined: "underline", hourglass: "hourglass", link_off: "link-2-off",
    contacts: "contact", local_cafe: "coffee", local_parking: "square-parking", map: "map", snowflake: "snowflake", tag: "tag", title: "heading", water_drop: "droplet",
  };

  const BASE = "/_gw/static/vendor/lucide/";
  const SELECTOR = "i.material-symbols, i.material-icons, i.material-symbols-outlined, span.material-symbols, span.material-icons";

  function apply(el) {
    const name = (el.textContent || "").trim();
    if (el.dataset.lucideFor === name) return;      // 이미 처리됨 (같은 이름)
    el.dataset.lucideFor = name;
    // 대응표에 없는 파일 종류 아이콘(file_*)은 기본 파일 아이콘으로
    const lucide = MAP[name] || (name.startsWith("file_") ? "file" : undefined);
    if (lucide) {
      el.dataset.lucide = lucide;
      el.style.setProperty("--nas-lucide", `url("${BASE}${lucide}.svg")`);
    } else {
      delete el.dataset.lucide;
      el.style.removeProperty("--nas-lucide");
    }
  }

  function scan(root) {
    if (root.nodeType !== 1) return;
    if (root.matches && root.matches(SELECTOR)) apply(root);
    for (const el of root.querySelectorAll(SELECTOR)) apply(el);
  }

  // Quantum은 아이콘 글자만 바꿔서 아이콘을 전환하기도 한다 (예: dark_mode ↔ light_mode) → characterData 도 감시
  new MutationObserver((records) => {
    for (const r of records) {
      if (r.type === "characterData") {
        const el = r.target.parentElement;
        if (el && el.matches(SELECTOR)) apply(el);
      } else {
        for (const n of r.addedNodes) {
          if (n.nodeType === 3 && n.parentElement && n.parentElement.matches(SELECTOR)) apply(n.parentElement);
          else scan(n);
        }
      }
    }
  }).observe(document.documentElement, { childList: true, subtree: true, characterData: true });

  scan(document.documentElement);
})();
