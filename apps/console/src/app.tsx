import { useEffect } from "react";
import { QueryClientProvider } from "@tanstack/react-query";
import { queryClient } from "@/lib/qc";
import { useAuth } from "@/lib/auth";
import { Login } from "@/pages/login";
import { Shell } from "@/components/layout/shell";
import { Dashboard } from "@/pages/dashboard";

export function App() {
  const token = useAuth((s) => s.token);
  const me = useAuth((s) => s.me);
  const loadMe = useAuth((s) => s.loadMe);

  useEffect(() => {
    if (token && !me) loadMe();
  }, [token, me, loadMe]);

  return (
    <QueryClientProvider client={queryClient}>
      {token ? (
        <Shell>
          <Dashboard />
        </Shell>
      ) : (
        <Login />
      )}
    </QueryClientProvider>
  );
}
