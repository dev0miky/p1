import { useMutation, useQuery, useQueryClient, type UseMutationOptions } from "@tanstack/react-query";
import { api, type ApiOptions } from "./api";
import { useAuth } from "./auth";

export function useApiQuery<T>(key: readonly unknown[], path: string, opts?: Omit<ApiOptions, "token">) {
  const token = useAuth((s) => s.token);
  return useQuery<T>({
    queryKey: key,
    queryFn: ({ signal }) => api<T>(path, { ...opts, token, signal }),
    enabled: !!token,
  });
}

export function useApiMutation<TResp, TInput>(
  path: string | ((input: TInput) => string),
  method: ApiOptions["method"] = "POST",
  options?: UseMutationOptions<TResp, Error, TInput> & { invalidate?: readonly unknown[] }
) {
  const token = useAuth((s) => s.token);
  const qc = useQueryClient();
  const { invalidate, onSuccess, ...rest } = options ?? {};
  return useMutation<TResp, Error, TInput>({
    mutationFn: (input) => {
      const p = typeof path === "function" ? path(input) : path;
      return api<TResp>(p, { method, token, body: method === "DELETE" ? undefined : input });
    },
    onSuccess: (...args) => {
      if (invalidate) qc.invalidateQueries({ queryKey: invalidate });
      onSuccess?.(...args);
    },
    ...rest,
  });
}
