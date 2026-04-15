import { create } from 'zustand';
import type { TunnelListItem } from '../types';
import * as tunnelsApi from '../api/tunnels';

interface TunnelState {
  tunnels: TunnelListItem[];
  loading: boolean;
  loadTunnels: () => Promise<void>;
}

export const useTunnelStore = create<TunnelState>((set) => ({
  tunnels: [],
  loading: false,
  loadTunnels: async () => {
    set({ loading: true });
    try {
      const data = await tunnelsApi.fetchTunnels();
      set({ tunnels: Array.isArray(data.tunnels) ? data.tunnels : [], loading: false });
    } catch {
      set({ loading: false });
    }
  },
}));
