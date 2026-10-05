// 암호화 보관함 화면. 암호화/복호화는 vault-worker.js에서, 파일 입출력은 Quantum API로 한다.
"use strict";

// Quantum 설정(server.sources[].name)과 같아야 한다.
const SOURCE = "files";
const CHUNK = 10 * 1024 * 1024; // Quantum 기본 분할 업로드 크기
const MIN_PASSPHRASE = 12;

const $ = (id) => document.getElementById(id);
let cwd = "/";
let selected = null; // 복호화할 .age 파일 경로

// ---- 상태 표시 ----
function status(text, isError = false) {
  $("status").textContent = text;
  $("status").classList.toggle("gw-error", isError);
}

function progress(done, total) {
  const p = $("progress");
  if (total == null) {
    p.hidden = true;
    return;
  }
  p.hidden = false;
  p.max = total || 1;
  p.value = done;
}

function busy(on) {
  for (const id of ["encrypt", "decrypt", "up", "file"]) $(id).disabled = on || (id === "decrypt" && !selected);
}

// ---- Quantum API ----
function apiURL(path, params) {
  return `${path}?${new URLSearchParams({ source: SOURCE, ...params })}`;
}

async function api(url, init = {}) {
  const res = await fetch(url, { credentials: "same-origin", ...init });
  if (res.redirected && new URL(res.url).pathname.startsWith("/_gw/")) {
    location.href = res.url; // 로그인 만료나 계정 전환으로 게이트웨이가 돌려보낸 경우
    throw new Error("다시 연결합니다");
  }
  if (!res.ok) throw new Error(`서버 오류 (${res.status})`);
  return res;
}

function joinPath(dir, name) {
  return (dir.endsWith("/") ? dir : dir + "/") + name;
}

async function listDir(path) {
  const res = await api(apiURL("/api/resources", { path }));
  return res.json();
}

async function uploadBlob(path, blob) {
  if (blob.size <= CHUNK) {
    await api(apiURL("/api/resources", { path, override: "false" }), { method: "POST", body: blob });
    return;
  }
  for (let offset = 0; offset < blob.size; offset += CHUNK) {
    await api(apiURL("/api/resources", { path, override: "false" }), {
      method: "POST",
      body: blob.slice(offset, offset + CHUNK),
      headers: { "X-File-Chunk-Offset": String(offset), "X-File-Total-Size": String(blob.size) },
    });
    progress(Math.min(offset + CHUNK, blob.size), blob.size);
  }
}

async function downloadBlob(path) {
  const res = await api(apiURL("/api/resources/download", { file: path }));
  return res.blob();
}

// ---- 암호화 Worker ----
const worker = new Worker("/_gw/static/vault-worker.js");
let jobSeq = 0;
const jobs = new Map();

worker.onmessage = (e) => {
  const { id, type } = e.data;
  const job = jobs.get(id);
  if (!job) return;
  if (type === "progress") {
    progress(e.data.done, e.data.total);
  } else if (type === "done") {
    jobs.delete(id);
    job.resolve(e.data.blob);
  } else {
    jobs.delete(id);
    job.reject(new Error(e.data.message));
  }
};

function runJob(op, passphrase, blob) {
  const id = ++jobSeq;
  return new Promise((resolve, reject) => {
    jobs.set(id, { resolve, reject });
    worker.postMessage({ id, op, passphrase, blob });
  });
}

// ---- 폴더 목록 ----
async function refresh() {
  $("cwd").textContent = cwd;
  $("up").disabled = cwd === "/";
  const list = $("listing");
  list.replaceChildren();
  let data;
  try {
    data = await listDir(cwd);
  } catch (err) {
    status(`폴더를 불러오지 못했습니다: ${err.message}`, true);
    return;
  }
  const folders = (data.folders || []).map((f) => f.name).sort();
  const ageFiles = (data.files || []).map((f) => f.name).filter((n) => n.endsWith(".age")).sort();

  for (const name of folders) {
    const li = document.createElement("li");
    li.textContent = `📁 ${name}`;
    li.addEventListener("click", () => {
      cwd = joinPath(cwd, name);
      select(null);
      refresh();
    });
    list.append(li);
  }
  for (const name of ageFiles) {
    const li = document.createElement("li");
    li.textContent = `🔒 ${name}`;
    li.addEventListener("click", () => {
      for (const other of list.children) other.classList.remove("selected");
      li.classList.add("selected");
      select(joinPath(cwd, name));
    });
    list.append(li);
  }
  if (!folders.length && !ageFiles.length) {
    const li = document.createElement("li");
    li.textContent = "(하위 폴더와 암호화 파일 없음)";
    list.append(li);
  }
}

function select(path) {
  selected = path;
  $("selected").textContent = path ? `선택: ${path}` : "위 목록에서 .age 파일을 선택하세요.";
  $("decrypt").disabled = !path;
}

$("up").addEventListener("click", () => {
  cwd = cwd.replace(/\/[^/]+\/?$/, "") || "/";
  select(null);
  refresh();
});

// ---- 암호화 업로드 ----
$("encrypt").addEventListener("click", async () => {
  const file = $("file").files[0];
  const pass = $("pass").value;
  if (!file) return status("파일을 선택하세요.", true);
  if (pass.length < MIN_PASSPHRASE) return status(`비밀번호는 ${MIN_PASSPHRASE}자 이상이어야 합니다.`, true);
  if (pass !== $("pass2").value) return status("비밀번호 확인이 일치하지 않습니다.", true);

  busy(true);
  try {
    status("암호화하는 중...");
    const encrypted = await runJob("encrypt", pass, file);
    status("업로드하는 중...");
    progress(0, encrypted.size);
    await uploadBlob(joinPath(cwd, file.name + ".age"), encrypted);
    status(`완료: ${file.name}.age`);
    $("file").value = "";
    $("pass2").value = "";
    await refresh();
  } catch (err) {
    status(`실패: ${err.message.includes("409") ? "같은 이름의 파일이 이미 있습니다." : err.message}`, true);
  } finally {
    progress(0, null);
    busy(false);
  }
});

// ---- 복호화 다운로드 ----
$("decrypt").addEventListener("click", async () => {
  const pass = $("pass").value;
  if (!selected || !pass) return status("파일과 비밀번호를 확인하세요.", true);

  busy(true);
  try {
    status("내려받는 중...");
    const encrypted = await downloadBlob(selected);
    status("복호화하는 중...");
    const plain = await runJob("decrypt", pass, encrypted);
    const name = selected.split("/").pop().replace(/\.age$/, "");
    const url = URL.createObjectURL(plain);
    const a = document.createElement("a");
    a.href = url;
    a.download = name;
    document.body.append(a);
    a.click();
    a.remove();
    setTimeout(() => URL.revokeObjectURL(url), 60_000);
    status(`완료: ${name}`);
  } catch (err) {
    const wrongPass = /no identity matched/i.test(err.message);
    status(wrongPass ? "비밀번호가 맞지 않습니다." : `실패: ${err.message}`, true);
  } finally {
    progress(0, null);
    busy(false);
  }
});

refresh();
