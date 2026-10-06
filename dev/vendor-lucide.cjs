// lucide-icons.js 의 대응표에 쓰인 Lucide 아이콘과 게이트웨이 화면에서 쓰는 아이콘만
// npm 패키지 lucide-static 에서 골라 gateway/web/static/vendor/lucide/ 로 복사한다.
// 사용: node dev/vendor-lucide.cjs <lucide-static 패키지 폴더>
//   예) npm pack lucide-static && tar -xzf lucide-static-*.tgz && node dev/vendor-lucide.cjs package
const fs = require("fs");
const path = require("path");

const pkg = process.argv[2];
if (!pkg) throw new Error("lucide-static 패키지 폴더를 인자로 주세요");
const icons = path.join(pkg, "icons");
const out = path.join(__dirname, "..", "gateway", "web", "static", "vendor", "lucide");

// 게이트웨이 화면(로그인, 보관함, 계정 메뉴 등)에서 직접 쓰는 아이콘
const GATEWAY = ["hard-drive", "log-out", "lock", "folder", "arrow-left-right", "triangle-alert", "monitor-smartphone"];

const src = fs.readFileSync(path.join(__dirname, "..", "gateway", "web", "static", "lucide-icons.js"), "utf8");
const mapped = [...src.matchAll(/:\s*"([a-z0-9-]+)"/g)].map((m) => m[1]);
const wanted = [...new Set([...mapped, ...GATEWAY])].sort();

// 기존 SVG만 지우고 LICENSE 등 다른 파일은 남긴다
fs.mkdirSync(out, { recursive: true });
for (const f of fs.readdirSync(out)) if (f.endsWith(".svg")) fs.rmSync(path.join(out, f));
const missing = [];
for (const name of wanted) {
  const file = path.join(icons, `${name}.svg`);
  if (!fs.existsSync(file)) {
    missing.push(name);
    continue;
  }
  fs.copyFileSync(file, path.join(out, `${name}.svg`));
}
const version = JSON.parse(fs.readFileSync(path.join(pkg, "package.json"), "utf8")).version;
console.log(`lucide-static ${version}: ${wanted.length - missing.length}개 복사`);
if (missing.length) {
  console.log("패키지에 없는 이름 (lucide-icons.js 에서 고쳐야 함):", missing.join(", "));
  process.exitCode = 1;
}
