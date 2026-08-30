import { QueryClient } from "@tanstack/react-query";
import { afterEach, describe, expect, it, vi } from "vitest";
import { authStatusQuery, refreshAuthStatus } from "./authQuery";

const unauthenticated = { initialized: true, env_managed: true, authenticated: false };
const authenticated = {
  initialized: true,
  env_managed: true,
  authenticated: true,
  session_expires_at: 1_800_000_000,
};

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("refreshAuthStatus", () => {
  it("replaces a still-fresh pre-login cache entry with the server session state", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData(authStatusQuery.queryKey, unauthenticated);

    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify(authenticated), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", fetchMock);

    await expect(refreshAuthStatus(queryClient)).resolves.toEqual(authenticated);
    expect(fetchMock).toHaveBeenCalledOnce();
    expect(queryClient.getQueryData(authStatusQuery.queryKey)).toEqual(authenticated);
  });

  it("keeps the previous cache entry when the status refresh fails", async () => {
    const queryClient = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    queryClient.setQueryData(authStatusQuery.queryKey, unauthenticated);

    vi.stubGlobal(
      "fetch",
      vi.fn().mockResolvedValue(
        new Response(JSON.stringify({ error: "temporary_failure" }), {
          status: 503,
          headers: { "Content-Type": "application/json" },
        }),
      ),
    );

    await expect(refreshAuthStatus(queryClient)).rejects.toMatchObject({
      status: 503,
      code: "temporary_failure",
    });
    expect(queryClient.getQueryData(authStatusQuery.queryKey)).toEqual(unauthenticated);
  });
});
