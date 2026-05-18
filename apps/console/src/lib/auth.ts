import { create } from "zustand";
import { persist } from "zustand/middleware";
import { api } from "./api";

export interface Me {
  user_id: number;
  email: string;
  role: string;
  tenant_id?: number;
  status: string;
}

interface LoginInput {
  email: string;
  password: string;
  tenant_slug?: string;
}

interface LoginResponse {
  token: string;
  user_id: number;
  tenant_id?: number;
  role: string;
}

interface AuthState {
  token: string | null;
  me: Me | null;
  login: (input: LoginInput) => Promise<void>;
  logout: () => void;
  loadMe: () => Promise<void>;
}

export const useAuth = create<AuthState>()(
  persist(
    (set, get) => ({
      token: null,
      me: null,
      async login(input) {
        const res = await api<LoginResponse>("/auth/login", { method: "POST", body: input });
        set({ token: res.token });
        await get().loadMe();
      },
      logout() {
        set({ token: null, me: null });
      },
      async loadMe() {
        const token = get().token;
        if (!token) return;
        try {
          const me = await api<Me>("/auth/me", { token });
          set({ me });
        } catch {
          set({ token: null, me: null });
        }
      },
    }),
    { name: "p1.auth", partialize: (s) => ({ token: s.token }) }
  )
);
