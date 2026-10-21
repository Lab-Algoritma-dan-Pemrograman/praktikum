// Pending input resolver for interactive mode
let pendingInputResolve = null;

// Mutable current run ID — updated before each RUN_INTERACTIVE
let currentInteractiveId = '';
let pyodideReady = false;
let moduleRegistered = false;

self.onmessage = async (event) => {
  const { type, code, id } = event.data;

  // Handle INPUT_RESPONSE for interactive mode
  if (type === 'INPUT_RESPONSE' && pendingInputResolve) {
    const resolver = pendingInputResolve;
    pendingInputResolve = null;
    resolver(event.data.value);
    return;
  }

  // Handle CANCEL for interactive mode (abort stuck input)
  if (type === 'CANCEL') {
    if (pendingInputResolve) {
      pendingInputResolve('__CANCELLED__');
      pendingInputResolve = null;
    }
    return;
  }

  if (type === 'INIT') {
    if (self.pyodide) {
      self.postMessage({ type: 'INIT_DONE' });
      return;
    }
    try {
      importScripts('https://cdn.jsdelivr.net/pyodide/v0.25.0/full/pyodide.js');
      self.pyodide = await self.loadPyodide({
        indexURL: 'https://cdn.jsdelivr.net/pyodide/v0.25.0/full/',
      });
      pyodideReady = true;
      self.postMessage({ type: 'INIT_DONE' });
    } catch (err) {
      self.postMessage({ type: 'INIT_ERROR', error: err.message });
    }
  } else if (type === 'RUN') {
    try {
      const pyodide = self.pyodide;
      if (!pyodide) {
        self.postMessage({ type: 'RUN_ERROR', id, error: 'Pyodide not initialized' });
        return;
      }

      const { input = '' } = event.data;
      
      await pyodide.runPythonAsync(`
import sys
import io
sys.stdout = io.StringIO()
sys.stdin = io.StringIO(${JSON.stringify(input)})
      `);

      await pyodide.runPythonAsync(code);
      const stdout = await pyodide.runPythonAsync('sys.stdout.getvalue()');
      
      self.postMessage({ type: 'RUN_DONE', id, output: stdout });
    } catch (err) {
      self.postMessage({ type: 'RUN_ERROR', id, error: err.message });
    }
  } else if (type === 'RUN_INTERACTIVE') {
    try {
      const pyodide = self.pyodide;
      if (!pyodide) {
        self.postMessage({ type: 'RUN_ERROR', id, error: 'Pyodide not initialized' });
        return;
      }

      // Update the current run ID (read by the module's readLine callback)
      currentInteractiveId = id;

      // Register terminal_io module ONCE (it reads currentInteractiveId dynamically)
      if (!moduleRegistered) {
        try {
          pyodide.registerJsModule('__terminal_io', {
            readLine: async (prompt) => {
              const output = await pyodide.runPythonAsync('_term_buf.getvalue()');
              return new Promise((resolve) => {
                self.postMessage({
                  type: 'INPUT_REQUEST',
                  id: currentInteractiveId,
                  prompt,
                  output,
                });
                pendingInputResolve = (value) => {
                  if (value === '__CANCELLED__') {
                    resolve('__cancelled__');
                  } else {
                    resolve(value);
                  }
                };
              });
            },
            flush: () => {},
          });
          moduleRegistered = true;
        } catch (e) {
          // Module already registered — that's fine, it uses currentInteractiveId
          moduleRegistered = true;
        }
      }

      // Setup stdout+stderr capture + async input function
      await pyodide.runPythonAsync(`
import sys
import io

_term_buf = io.StringIO()
sys.stdout = _term_buf
sys.stderr = _term_buf

import __terminal_io

class _CancelledError(Exception):
    pass

async def _async_input(prompt_str=""):
    result = await __terminal_io.readLine(prompt_str)
    if result == "__cancelled__":
        raise _CancelledError("Program cancelled by user")
    return result
`);

      // Build the wrapped code: replace input() with (await _async_input())
      // Kurung wajib: tanpa itu `input("x").lower()` jadi
      // `await _async_input("x").lower()` — .lower() jalan di coroutine, bukan
      // di string, dan meledak "AttributeError: 'coroutine' object has no
      // attribute 'lower'". Sama untuk int(input()), .strip(), .split().
      const userLines = code.split('\n');
      const wrappedLines = userLines.map(line => {
        if (line.trim() === '' || line.trim().startsWith('#')) return line;
        return '    ' + line.replace(/\binput\(/g, '(await _async_input(').replace(/(\(await _async_input\((?:[^()]|\([^()]*\))*\))/g, '$1)');
      });

      const wrappedCode = `
async def __interactive_run():
${wrappedLines.join('\n')}

await __interactive_run()
`;

      await pyodide.runPythonAsync(wrappedCode);

      // Final flush
      const stdout = await pyodide.runPythonAsync('_term_buf.getvalue()');
      self.postMessage({ type: 'RUN_DONE', id, output: stdout });
    } catch (err) {
      // Check if this was a user cancellation
      if (err.message && err.message.includes('cancelled')) {
        self.postMessage({ type: 'RUN_CANCELLED', id });
      } else {
        self.postMessage({ type: 'RUN_ERROR', id, error: err.message });
      }
    }
  }
};
