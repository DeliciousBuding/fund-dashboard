import { request, type FullConfig } from "@playwright/test";
import { mkdir } from "node:fs/promises";
import path from "node:path";

export default async function globalSetup(_config: FullConfig) {
  const baseURL = process.env.E2E_BASE_URL ?? "http://127.0.0.1:8080";
  const password = process.env.E2E_PASSWORD ?? "ci-smoke-password-1";
  const statePath = path.resolve("test-results/e2e-auth.json");
  await mkdir(path.dirname(statePath), { recursive: true });

  const api = await request.newContext({ baseURL });
  try {
    const statusResponse = await api.get("/api/auth/status");
    if (!statusResponse.ok()) {
      throw new Error(`auth status failed: ${statusResponse.status()} ${await statusResponse.text()}`);
    }
    const status = (await statusResponse.json()) as { initialized: boolean };
    const endpoint = status.initialized ? "/api/auth/login" : "/api/auth/setup";
    const expectedStatus = status.initialized ? 200 : 201;
    const response = await api.post(endpoint, {
      data: { password },
      headers: { "X-Fund-Request": "playwright" },
    });
    if (response.status() !== expectedStatus) {
      throw new Error(`${endpoint} failed: ${response.status()} ${await response.text()}`);
    }
    await api.storageState({ path: statePath });
  } finally {
    await api.dispose();
  }
}
