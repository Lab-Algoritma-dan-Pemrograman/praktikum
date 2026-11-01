/*
 * Copyright 2020 WebAssembly Community Group participants
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

self.importScripts('shared.js?v=8');

let api;
let port;
let canvas;
let ctx2d;

const apiOptions = {
  async readBuffer(filename) {
    const response = await fetch(filename);
    return response.arrayBuffer();
  },

  async compileStreaming(filename) {
    // TODO: make compileStreaming work. It needs the server to use the
    // application/wasm mimetype.
    if (false && WebAssembly.compileStreaming) {
      return WebAssembly.compileStreaming(fetch(filename));
    } else {
      const response = await fetch(filename);
      return WebAssembly.compile(await response.arrayBuffer());
    }
  },

  hostWrite(s) { port.postMessage({id : 'write', data : s}); }
};

let currentApp = null;
let currentModule = null;
let lastCompiledCode = '';

const onAnyMessage = async event => {
  switch (event.data.id) {
  case 'constructor':
    port = event.data.data;
    port.onmessage = onAnyMessage;
    // SAB untuk blocking stdin (opsional): kalau main-thread mengirim, pakai
    // mode menunggu input nyata (seperti terminal asli). Tanpa SAB, fallback
    // ke perilaku lama (EOF saat buffer habis).
    apiOptions.sab = event.data.sab || null;
    api = new API(apiOptions);
    api.ready.then(() => {
      port.postMessage({ id: 'init_done' });
    }).catch(err => {
      port.postMessage({ id: 'init_error', error: err.message || String(err) });
    });
    break;

  case 'setShowTiming':
    api.showTiming = event.data.data;
    break;

  case 'compileToAssembly': {
    const responseId = event.data.responseId;
    let output = null;
    let transferList;
    try {
      output = await api.compileToAssembly(event.data.data);
    } finally {
      port.postMessage({id : 'runAsync', responseId, data : output},
                       transferList);
    }
    break;
  }

  case 'compileTo6502': {
    const responseId = event.data.responseId;
    let output = null;
    let transferList;
    try {
      output = await api.compileTo6502(event.data.data);
    } finally {
      port.postMessage({id : 'runAsync', responseId, data : output},
                       transferList);
    }
    break;
  }

  case 'compileLinkRun':
    if (currentApp) {
      // console.log('First, disallowing rAF from previous app.');
      // Stop running rAF on the previous app, if any.
      currentApp.allowRequestAnimationFrame = false;
    }
    try {
      currentApp = await api.compileLinkRun(event.data.data);
    } finally {
      port.postMessage({id : 'runAsync', responseId : event.data.responseId,
                        data : currentApp});
      // console.log(`finished compileLinkRun. currentApp = ${currentApp}.`);
    }
    break;

  case 'compile': {
    const responseId = event.data.responseId;
    let success = false;
    let error = null;
    try {
      const contents = event.data.data;
      if (contents === lastCompiledCode && currentModule) {
        success = true;
      } else {
        const input = 'main.c';
        const obj = 'main.o';
        const wasm = 'main.wasm';
        
        await api.compile({input, contents, obj});
        await api.link(obj, wasm);
        
        const buffer = api.memfs.getFileContents(wasm);
        currentModule = await WebAssembly.compile(buffer);
        lastCompiledCode = contents;
        success = true;
      }
    } catch (e) {
      error = e.message || String(e);
      lastCompiledCode = '';
    } finally {
      port.postMessage({id: 'runAsync', responseId, data: { success, error }});
    }
    break;
  }

  case 'run': {
    const responseId = event.data.responseId;
    let success = false;
    let error = null;
    try {
      if (!currentModule) {
        throw new Error("No compiled module found. Please compile first.");
      }

      // Default stdin "\n" saat kosong: di sandbox WASM ini stdin non-blocking,
      // jadi getchar() langsung EOF. Pola `while (getchar() != '\n')` yang umum
      // di materi jadi infinite loop. Dengan default newline, loop itu keluar
      // (seperti user menekan Enter). Program yang butuh input berulang tetap
      // butuh input nyata dari user.
      // Mode SAB hanya aktif bila main-thread tidak meminta mode legacy (pakai
      // untuk jalur tanpa terminal interaktif, mis. validasi soal di LessonPage).
      const sabActive = !!api.memfs.sab && !event.data.legacy;
      api.memfs.sabActive = sabActive;
      // PENTING: saat SAB aktif, stdin harus KOSONG (bukan default '\n') —
      // kalau tidak, program baca newline placeholder lalu selesai tanpa
      // menunggu input asli dari user.
      const stdin = event.data.data ?? (sabActive ? '' : '\n');
      api.memfs.setStdinStr(stdin);
      api.memfs.waitingForInput = false; // Reset before run
      
      const wasm = 'main.wasm';
      if (currentApp) {
        currentApp.allowRequestAnimationFrame = false;
      }
      currentApp = await api.run(currentModule, wasm);
      success = true;
    } catch (e) {
      error = e.message || String(e);
    } finally {
      const stdin = event.data.data || '';
      const inputsCount = stdin ? (stdin.endsWith('\n') ? stdin.slice(0, -1).split('\n').length : stdin.split('\n').length) : 0;
      
      const len = api.memfs.stdoutLenAtInputRequest || 0;
      let offsets = api.memfs.inputOffsets || [];
      const rawWaiting = api.memfs.waitingForInput || false;
      const sabBlocked = api.memfs.sabBlocked || false;
      if (rawWaiting && !offsets.includes(len)) {
        offsets = [...offsets, len];
      }
      
      if (api.memfs) api.memfs.sabActive = false;
      port.postMessage({id: 'runAsync', responseId, data: { success, error, waitingForInput: rawWaiting, sabBlocked, stdoutLenAtInputRequest: len, inputOffsets: offsets }});
    }
    break;
  }

  case 'postCanvas':
    canvas = event.data.data;
    ctx2d = canvas.getContext('2d');
    break;
  }
};

self.onmessage = onAnyMessage;
