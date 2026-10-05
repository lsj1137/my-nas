// 암호화/복호화 전용 Web Worker. scrypt 계산이 무거워서 화면 스레드와 분리한다.
// 비밀번호와 평문은 이 브라우저 안에서만 다뤄지고 서버로 전송되지 않는다.
"use strict";
importScripts("/_gw/static/vendor/age-0.3.1.min.js");

const PROGRESS_STEP = 4 * 1024 * 1024;

// 입력 스트림이 읽힌 바이트 수를 화면 스레드에 알려준다.
function withProgress(stream, id, total) {
  let done = 0;
  let lastReported = 0;
  return stream.pipeThrough(
    new TransformStream({
      transform(chunk, controller) {
        done += chunk.byteLength;
        if (done - lastReported >= PROGRESS_STEP || done === total) {
          lastReported = done;
          postMessage({ id, type: "progress", done, total });
        }
        controller.enqueue(chunk);
      },
    }),
  );
}

self.onmessage = async (event) => {
  const { id, op, passphrase, blob } = event.data;
  try {
    const input = withProgress(blob.stream(), id, blob.size);
    let output;
    if (op === "encrypt") {
      const e = new age.Encrypter();
      e.setPassphrase(passphrase);
      output = await e.encrypt(input);
    } else if (op === "decrypt") {
      const d = new age.Decrypter();
      d.addPassphrase(passphrase);
      output = await d.decrypt(input); // 비밀번호가 틀리면 여기서 바로 실패한다
    } else {
      throw new Error("unknown op");
    }
    const result = await new Response(output).blob();
    postMessage({ id, type: "done", blob: result });
  } catch (err) {
    postMessage({ id, type: "error", message: String((err && err.message) || err) });
  }
};
