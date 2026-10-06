// 로컬 개발 환경을 실제 브라우저(설치된 Chrome)로 확인하는 E2E 테스트.
// 전제: dev/gateway.yaml 의 ip_map 이 켜져 있음 (기본값) → 접속하면 shared 로 들어감.
// 실행: cd dev/e2e && npm install && npm test
// Chrome 경로가 다르면: CHROME_PATH="..." npm test
const puppeteer = require("puppeteer-core");
const fs = require("fs");
const path = require("path");
const crypto = require("crypto");

const B = "https://nas.localhost:8443";
const CHROME = process.env.CHROME_PATH || "C:/Program Files/Google/Chrome/Application/chrome.exe";
const OWNER = { user: "owner", pass: "dev-password" };
const VAULT_PASS = "a very long test passphrase";
const WORK = path.join(__dirname, ".work");

const results = [];
const check = (name, pass, detail = "") => results.push({ name, pass, detail });

const menuUser = (page) => page.waitForSelector("#nas-overlay .nas-name", { timeout: 15000 }).then((e) => e.evaluate((x) => x.textContent)).catch(() => null);

async function api(page, method, url, body) {
  return page.evaluate(async (method, url, body) => {
    const res = await fetch(url, { method, body, credentials: "same-origin" });
    return res.status;
  }, method, url, body);
}

async function login(page, { user, pass }) {
  await page.goto(B + "/_gw/login", { waitUntil: "networkidle2" });
  await page.type('input[name="username"]', user);
  await page.type('input[name="password"]', pass);
  await Promise.all([page.waitForNavigation({ waitUntil: "networkidle2" }), page.click('button[type="submit"]')]);
}

(async () => {
  fs.rmSync(WORK, { recursive: true, force: true });
  fs.mkdirSync(path.join(WORK, "downloads"), { recursive: true });

  const browser = await puppeteer.launch({ executablePath: CHROME, headless: true, args: ["--ignore-certificate-errors", "--lang=ko-KR"] });
  const page = await browser.newPage();
  await page.setViewport({ width: 1280, height: 800 });
  const consoleErrors = [];
  page.on("console", (m) => { if (m.type() === "error") consoleErrors.push(m.text()); });
  const cdp = await page.createCDPSession();
  await cdp.send("Browser.setDownloadBehavior", { behavior: "allow", downloadPath: path.join(WORK, "downloads") });

  // 1) IP 자동 접속
  await page.goto(B + "/", { waitUntil: "networkidle2" });
  const who1 = await menuUser(page);
  check("IP 자동 접속 → 계정 메뉴에 shared 표시", who1 === "shared", `표시: ${who1}`);

  // 2) 삭제 확인 창 경고 (테스트 파일을 만들고 삭제 창을 띄운 뒤 취소)
  const testFile = `e2e-${Date.now()}.txt`;
  await api(page, "POST", `/api/resources?source=files&path=/${testFile}&override=true`, "e2e");
  await page.reload({ waitUntil: "networkidle2" });
  const item = await page.waitForSelector(`[aria-label="${testFile}"]`, { timeout: 10000 }).catch(() => null);
  let warning = null;
  if (item) {
    await item.click();
    await page.keyboard.press("Delete");
    warning = await page.waitForSelector('[aria-label="delete-prompt"] .nas-delete-warning', { timeout: 5000 }).then((e) => e.evaluate((x) => x.textContent)).catch(() => null);
    await page.keyboard.press("Escape");
    await page.waitForSelector('[aria-label="delete-prompt"]', { hidden: true, timeout: 5000 }).catch(() => {});
  }
  check("삭제 확인 창에 영구 삭제 경고", !!warning, warning || "경고를 찾지 못함");
  await api(page, "DELETE", `/api/resources?source=files&path=/${testFile}`);

  // 3) 계정 메뉴로 전환
  await page.click("#nas-overlay .nas-toggle");
  await page.waitForFunction(() => getComputedStyle(document.querySelector("#nas-overlay .nas-menu")).opacity === "1");
  await Promise.all([page.waitForNavigation(), page.click('#nas-overlay a[href="/_gw/login"]')]);
  await page.type('input[name="username"]', OWNER.user);
  await page.type('input[name="password"]', OWNER.pass);
  await Promise.all([page.waitForNavigation({ waitUntil: "networkidle2" }), page.click('button[type="submit"]')]);
  const who2 = await menuUser(page);
  check("계정 전환 shared → owner", who2 === OWNER.user, `표시: ${who2}`);

  // 4) Quantum 로그아웃 버튼이 가는 주소 (도메인 없이 /_gw/logout)
  const logoutUrl = await page.evaluate(async () => (await (await fetch("/api/auth/logout", { method: "POST", credentials: "same-origin" })).json()).logoutUrl);
  check("Quantum 로그아웃 주소가 /_gw/logout", logoutUrl === "/_gw/logout", logoutUrl);
  await login(page, OWNER); // 위 API 호출로 Quantum 세션이 끊겼을 수 있으니 다시 연결

  // 5) 암호화 보관함 왕복
  const plain = crypto.randomBytes(3 * 1024 * 1024);
  const name = `secret-${Date.now()}.bin`;
  const src = path.join(WORK, name);
  fs.writeFileSync(src, plain);
  await page.goto(B + "/_gw/vault", { waitUntil: "networkidle2" });
  await page.waitForFunction(() => document.querySelectorAll("#listing li").length > 0);
  await (await page.$("#file")).uploadFile(src);
  await page.type("#pass", VAULT_PASS);
  await page.type("#pass2", VAULT_PASS);
  await page.click("#encrypt");
  await page.waitForFunction(() => /완료|실패/.test(document.getElementById("status").textContent), { timeout: 60000 });
  const encStatus = await page.$eval("#status", (e) => e.textContent);
  check("암호화 업로드", encStatus.startsWith("완료"), encStatus);

  await page.waitForFunction((n) => [...document.querySelectorAll("#listing li")].some((li) => li.textContent === n + ".age"), {}, name);
  await page.evaluate((n) => [...document.querySelectorAll("#listing li")].find((li) => li.textContent === n + ".age").click(), name);
  await page.$eval("#pass", (e) => (e.value = "wrong passphrase!!"));
  await page.click("#decrypt");
  await page.waitForFunction(() => /완료|실패|오류/.test(document.getElementById("status").textContent), { timeout: 60000 });
  const wrong = await page.$eval("#status", (e) => e.textContent);
  check("틀린 비밀번호 거부", wrong.includes("비밀번호 오류"), wrong);

  await page.$eval("#pass", (e, p) => (e.value = p), VAULT_PASS);
  await page.click("#decrypt");
  await page.waitForFunction(() => document.getElementById("status").textContent.startsWith("완료"), { timeout: 60000 });
  const dl = path.join(WORK, "downloads", name);
  for (let i = 0; i < 50 && !fs.existsSync(dl); i++) await new Promise((r) => setTimeout(r, 200));
  check("복호화 결과가 원본과 동일", fs.existsSync(dl) && Buffer.compare(fs.readFileSync(dl), plain) === 0);
  await api(page, "DELETE", `/api/resources?source=files&path=/${name}.age`);

  // 6) 로그아웃 → shared 복귀
  await page.goto(B + "/", { waitUntil: "networkidle2" });
  await page.waitForSelector("#nas-overlay .nas-toggle");
  await page.click("#nas-overlay .nas-toggle");
  await page.waitForFunction(() => getComputedStyle(document.querySelector("#nas-overlay .nas-menu")).opacity === "1");
  await Promise.all([page.waitForNavigation({ waitUntil: "networkidle2" }), page.click("#nas-overlay .nas-icon-logout")]);
  const who3 = await menuUser(page);
  check("로그아웃 → IP 자동 계정(shared)으로 복귀", who3 === "shared", `표시: ${who3}`);

  check("브라우저 콘솔 오류 없음 (CSP 위반 등)", consoleErrors.length === 0, consoleErrors.slice(0, 3).join(" | "));
  await browser.close();
})()
  .catch((e) => check("테스트 실행", false, e.message))
  .finally(() => {
    for (const r of results) console.log(`${r.pass ? "✅" : "❌"} ${r.name}${r.detail ? " — " + r.detail : ""}`);
    const failed = results.filter((r) => !r.pass).length;
    console.log(failed ? `\n${failed}개 실패` : "\n모두 통과");
    process.exitCode = failed ? 1 : 0;
  });
