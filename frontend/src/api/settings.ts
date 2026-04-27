import apiClient from './client';
import type {
  GeneralResponse,
  ClientBuildSettingsResponse,
  ClientBuildSettingsPayload,
  PlayerClientBuildSettingsResponse,
  GenerateClientReq,
} from '../types';

export async function fetchClientBuildSettings(): Promise<ClientBuildSettingsResponse> {
  const { data } = await apiClient.post<ClientBuildSettingsResponse>(
    '/api/client_build_settings',
    {},
  );
  return data;
}

export async function updateClientBuildSettings(
  settings: ClientBuildSettingsPayload,
): Promise<GeneralResponse> {
  const { data } = await apiClient.post<GeneralResponse>(
    '/api/update_client_build_settings',
    settings,
  );
  return data;
}

export async function fetchPlayerClientBuildSettings(
  playerId: number,
): Promise<PlayerClientBuildSettingsResponse> {
  const { data } = await apiClient.post<PlayerClientBuildSettingsResponse>(
    '/api/player_client_build_settings',
    { player_id: playerId },
  );
  return data;
}

export async function generateClient(req: GenerateClientReq): Promise<{
  success: boolean;
  blob?: Blob;
  filename?: string;
  error?: string;
}> {
  try {
    const response = await fetch(window.location.origin + '/api/generate_client', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(req),
      credentials: 'include',
    });
    const contentType = response.headers.get('Content-Type') || '';
    if (contentType.includes('application/json')) {
      const data = await response.json();
      if (data.code === 10086) {
        window.dispatchEvent(new CustomEvent('auth-expired'));
        return { success: false, error: 'auth expired' };
      }
      return { success: false, error: data.msg || 'Unknown error' };
    }
    const blob = await response.blob();
    const disposition = response.headers.get('Content-Disposition') || '';
    let filename = '';
    const utf8Match = disposition.match(/filename\*=UTF-8''([^;]+)/i);
    if (utf8Match?.[1]) {
      filename = decodeURIComponent(utf8Match[1]);
    } else {
      const match = disposition.match(/filename="([^"]+)"/i);
      filename = match?.[1] || `gpipe-client-${req.player_id}`;
    }
    return { success: true, blob, filename };
  } catch (err) {
    return { success: false, error: String(err) };
  }
}
