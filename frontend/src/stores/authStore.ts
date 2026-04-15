import { create } from 'zustand';
import * as authApi from '../api/auth';

interface AuthState {
  isLoggedIn: boolean;
  checking: boolean;
  setLoggedIn: (v: boolean) => void;
  checkLoginStatus: () => Promise<boolean>;
  login: (username: string, password: string) => Promise<{ success: boolean; msg?: string }>;
  logout: () => Promise<void>;
}

export const useAuthStore = create<AuthState>((set) => ({
  isLoggedIn: false,
  checking: true,
  setLoggedIn: (v) => set({ isLoggedIn: v }),
  checkLoginStatus: async () => {
    set({ checking: true });
    try {
      const data = await authApi.testAuth();
      const ok = data.code === 0;
      set({ isLoggedIn: ok, checking: false });
      return ok;
    } catch {
      set({ isLoggedIn: false, checking: false });
      return false;
    }
  },
  login: async (username, password) => {
    try {
      const data = await authApi.login({ username, password });
      if (data.code === 0) {
        set({ isLoggedIn: true });
        return { success: true };
      }
      return { success: false, msg: data.msg };
    } catch {
      return { success: false, msg: 'Network error' };
    }
  },
  logout: async () => {
    try {
      await authApi.logout();
    } catch {
      // ignore
    }
    set({ isLoggedIn: false });
  },
}));
