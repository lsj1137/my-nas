# vendor

외부 CDN을 쓰지 않고 게이트웨이가 직접 제공하는 외부 파일들.

| 파일 | 출처 | 라이선스 | 비고 |
|---|---|---|---|
| `age-0.3.1.min.js` | https://github.com/FiloSottile/typage/releases/download/v0.3.1/age-0.3.1.min.js | BSD-3-Clause (번들된 @noble, @scure는 MIT) | SHA-256 `112306e7bc83bf28f47e45f632c5989c0ea979cdc7d0e5c71823b3eae7f086ae` |
| `fonts/PretendardVariable.woff2` | npm `pretendard@1.3.9` `dist/web/variable/woff2/` | SIL OFL 1.1 (`fonts/OFL.txt`) | SHA-256 `9599f12fd42fc0bce1cd50b47a0c022e108d7aa64dd0d1bb0ed44f3282d900b4` |
| `lucide/*.svg` | npm `lucide-static@1.52.0` `icons/` | ISC (`lucide/LICENSE`) | 실제로 쓰는 아이콘만 복사 (`dev/vendor-lucide.cjs`) |

## 업데이트

- age: 릴리스 자산을 새로 내려받고 위 표의 버전과 체크섬을 갱신한다.
- Pretendard: `npm pack pretendard` 후 같은 경로의 woff2를 복사하고 체크섬을 갱신한다.
- Lucide: `npm pack lucide-static && tar -xzf lucide-static-*.tgz && node dev/vendor-lucide.cjs package`
  - 아이콘 대응표(`static/lucide-icons.js`)에 새 이름을 추가했을 때도 이 명령을 다시 실행한다.
