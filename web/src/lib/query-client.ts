import { QueryClient } from "@tanstack/react-query";
import { HttpError } from "./http-client";

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Serve cached data while re-fetching in the background.
      staleTime: 30_000,
      // Keep unused cache entries for 5 minutes.
      gcTime: 5 * 60_000,
      // Retry failed requests up to 3 times, but never retry auth/client errors.
      retry: (failureCount, error) => {
        if (error instanceof HttpError && error.status < 500) return false;
        return failureCount < 3;
      },
      retryDelay: (attempt) => Math.min(1000 * 2 ** attempt, 30_000),
      refetchOnWindowFocus: true,
    },
    mutations: {
      // Mutations don't retry by default — callers opt in explicitly.
      retry: false,
    },
  },
});
