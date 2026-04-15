import apiClient from './client';
import type { GeneralResponse, LoginReq } from '../types';

export async function login(req: LoginReq): Promise<GeneralResponse> {
  const { data } = await apiClient.post<GeneralResponse>('/api/login', req);
  return data;
}

export async function logout(): Promise<GeneralResponse> {
  const { data } = await apiClient.post<GeneralResponse>('/api/logout', {});
  return data;
}

export async function testAuth(): Promise<GeneralResponse> {
  const { data } = await apiClient.post<GeneralResponse>('/api/test_auth', {});
  return data;
}
