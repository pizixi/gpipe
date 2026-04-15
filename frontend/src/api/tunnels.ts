import apiClient from './client';
import type {
  GeneralResponse,
  TunnelListResponse,
  TunnelAddReq,
  TunnelUpdateReq,
  TunnelRemoveReq,
} from '../types';

export async function fetchTunnels(): Promise<TunnelListResponse> {
  const { data } = await apiClient.post<TunnelListResponse>('/api/tunnel_list', {
    page_number: 0,
    page_size: 0,
  });
  return data;
}

export async function addTunnel(req: TunnelAddReq): Promise<GeneralResponse> {
  const { data } = await apiClient.post<GeneralResponse>('/api/add_tunnel', req);
  return data;
}

export async function updateTunnel(req: TunnelUpdateReq): Promise<GeneralResponse> {
  const { data } = await apiClient.post<GeneralResponse>('/api/update_tunnel', req);
  return data;
}

export async function removeTunnel(req: TunnelRemoveReq): Promise<GeneralResponse> {
  const { data } = await apiClient.post<GeneralResponse>('/api/remove_tunnel', req);
  return data;
}
