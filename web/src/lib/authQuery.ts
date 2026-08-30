import { type QueryClient, queryOptions } from "@tanstack/react-query";
import { type AuthStatus, fetchAuthStatus } from "./auth";

export const authStatusQuery = queryOptions({
  queryKey: ["auth-status"],
  queryFn: ({ signal }) => fetchAuthStatus(signal),
  staleTime: 60 * 1000,
});

/**
 * Authentication mutations change server state immediately, while route guards
 * may still hold a fresh pre-mutation cache entry. Fetch first, then replace the
 * cache atomically so the next guard observes the new session state.
 */
export async function refreshAuthStatus(queryClient: QueryClient): Promise<AuthStatus> {
  const status = await fetchAuthStatus();
  queryClient.setQueryData(authStatusQuery.queryKey, status);
  return status;
}
