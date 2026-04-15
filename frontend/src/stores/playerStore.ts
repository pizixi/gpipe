import { create } from 'zustand';
import type { PlayerListItem } from '../types';
import * as playersApi from '../api/players';

interface PlayerState {
  players: PlayerListItem[];
  loading: boolean;
  loadPlayers: () => Promise<void>;
}

export const usePlayerStore = create<PlayerState>((set) => ({
  players: [],
  loading: false,
  loadPlayers: async () => {
    set({ loading: true });
    try {
      const data = await playersApi.fetchPlayers();
      set({ players: Array.isArray(data.players) ? data.players : [], loading: false });
    } catch {
      set({ loading: false });
    }
  },
}));
