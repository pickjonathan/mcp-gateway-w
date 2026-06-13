import { QueryClient } from '@tanstack/react-query'

// Shared query cache. Short stale time + light retry; health/usage views opt into
// periodic refetch via their own hooks (US6).
export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 10_000,
      retry: 1,
      refetchOnWindowFocus: false,
    },
  },
})
