import apiClient from './client';
import type {
  GeneralResponse,
  PlayerListResponse,
  PlayerAddReq,
  PlayerUpdateReq,
  PlayerRemoveReq,
} from '../types';

export async function fetchPlayers(): Promise<PlayerListResponse> {
  const { data } = await apiClient.post<PlayerListResponse>('/api/player_list', {
    page_number: 0,
    page_size: 0,
  });
  return data;
}

export async function addPlayer(req: PlayerAddReq): Promise<GeneralResponse> {
  const { data } = await apiClient.post<GeneralResponse>('/api/add_player', req);
  return data;
}

export async function updatePlayer(req: PlayerUpdateReq): Promise<GeneralResponse> {
  const { data } = await apiClient.post<GeneralResponse>('/api/update_player', req);
  return data;
}

export async function removePlayer(req: PlayerRemoveReq): Promise<GeneralResponse> {
  const { data } = await apiClient.post<GeneralResponse>('/api/remove_player', req);
  return data;
}
