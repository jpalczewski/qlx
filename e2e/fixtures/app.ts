import { test as base, expect } from '@playwright/test';
import { execFileSync, spawn, ChildProcess } from 'child_process';
import * as fs from 'fs';
import * as os from 'os';
import * as path from 'path';
import * as net from 'net';

const PROJECT_ROOT = path.resolve(__dirname, '../..');
const BINARY_PATH = path.join(PROJECT_ROOT, 'qlx-e2e-test');

async function getAvailablePort(): Promise<number> {
  return new Promise((resolve, reject) => {
    const srv = net.createServer();
    srv.listen(0, () => {
      const port = (srv.address() as net.AddressInfo).port;
      srv.close(() => resolve(port));
    });
    srv.on('error', reject);
  });
}

function buildBinary() {
  if (fs.existsSync(BINARY_PATH)) return;
  execFileSync('go', ['build', '-o', BINARY_PATH, './cmd/qlx/'], {
    cwd: PROJECT_ROOT,
    stdio: 'inherit',
  });
}

type AppFixtures = {
  /** Override page to set lang=pl cookie before each test */
  page: import('@playwright/test').Page;
};

type AppWorkerFixtures = {
  app: { baseURL: string; port: number; dataDir: string };
};

export const test = base.extend<AppFixtures, AppWorkerFixtures>({
  page: async ({ page, app }, use) => {
    // Set lang=pl cookie so i18n is deterministic regardless of system locale
    await page.context().addCookies([{
      name: 'lang',
      value: 'pl',
      domain: '127.0.0.1',
      path: '/',
    }]);
    // SSE (printer status) keeps a connection open indefinitely, preventing
    // the 'load' event from firing on slow CI runners. Override goto to
    // default to 'domcontentloaded' so tests don't need to specify it.
    const originalGoto = page.goto.bind(page);
    page.goto = (url: string, options?: Parameters<typeof page.goto>[1]) => {
      return originalGoto(url, { waitUntil: 'domcontentloaded', ...options });
    };
    await use(page);
  },
  app: [async ({}, use) => {
    buildBinary();

    const port = await getAvailablePort();
    const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), 'qlx-e2e-'));
    const logPath = path.join(dataDir, 'server.log');
    const logFd = fs.openSync(logPath, 'w');

    const proc: ChildProcess = spawn(BINARY_PATH, [
      '--port', String(port),
      '--host', '127.0.0.1',
      '--data', dataDir,
    ], {
      cwd: PROJECT_ROOT,
      stdio: ['ignore', logFd, logFd],
    });

    // Detect early crash
    let crashed = false;
    let exitCode: number | null = null;
    proc.on('exit', (code) => { crashed = true; exitCode = code; });

    // Wait for server to be ready
    const startTime = Date.now();
    const timeout = 20_000;
    let ready = false;
    while (Date.now() - startTime < timeout) {
      if (crashed) {
        const log = fs.readFileSync(logPath, 'utf-8').slice(-2000);
        throw new Error(`QLX server crashed (exit ${exitCode}) on port ${port}.\nLast log:\n${log}`);
      }
      try {
        const res = await fetch(`http://127.0.0.1:${port}/`);
        if (res.ok) { ready = true; break; }
      } catch {
        // server not ready yet
      }
      await new Promise(r => setTimeout(r, 100));
    }
    if (!ready) {
      const log = fs.readFileSync(logPath, 'utf-8').slice(-2000);
      throw new Error(`QLX server failed to start on port ${port} within ${timeout}ms.\nLast log:\n${log}`);
    }

    await use({ baseURL: `http://127.0.0.1:${port}`, port, dataDir });

    // Teardown
    proc.kill('SIGTERM');
    await new Promise<void>((resolve) => {
      proc.on('exit', () => resolve());
      setTimeout(() => { proc.kill('SIGKILL'); resolve(); }, 3000);
    });
    fs.closeSync(logFd);
    fs.rmSync(dataDir, { recursive: true, force: true });
  }, { scope: 'worker' }],
});

export { expect };
