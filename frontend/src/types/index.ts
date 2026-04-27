// Types aligned with internal/web/types.go

export interface LoginReq {
  username: string;
  password: string;
}

export interface GeneralResponse {
  code: number;
  msg: string;
}

export interface PlayerListRequest {
  page_number: number;
  page_size: number;
}

export interface PlayerListItem {
  id: number;
  remark: string;
  key: string;
  create_time: string;
  last_online_time: string | null;
  online: boolean;
}

export interface PlayerListResponse {
  players: PlayerListItem[];
  cur_page_number: number;
  total_count: number;
}

export interface ClientBuildSettingsPayload {
  server: string;
  enable_tls: boolean;
  tls_server_name: string;
  use_shadowsocks: boolean;
  ss_server: string;
  ss_method: string;
  ss_password: string;
}

export interface ClientBuildSettingsResponse {
  settings: ClientBuildSettingsPayload;
}

export interface PlayerClientBuildSettingsResponse {
  settings: ClientBuildSettingsPayload;
  customized: boolean;
}

export interface GenerateClientReq {
  player_id: number;
  target: string;
  settings?: ClientBuildSettingsPayload;
}

export interface PlayerRemoveReq {
  id: number;
}

export interface PlayerAddReq {
  remark: string;
  key: string;
}

export interface PlayerUpdateReq {
  id: number;
  remark: string;
  key: string;
}

export interface TunnelListRequest {
  page_number: number;
  page_size: number;
  player_id?: string;
}

export interface TunnelListItem {
  id: number;
  source: string;
  endpoint: string;
  enabled: boolean;
  runtime_status: 'disabled' | 'waiting' | 'running' | 'failed' | 'starting' | 'unverified' | string;
  runtime_running: boolean;
  runtime_message: string;
  sender: number;
  receiver: number;
  description: string;
  tunnel_type: number;
  password: string;
  username: string;
  is_compressed: boolean;
  encryption_method: string;
  custom_mapping: Record<string, string>;
}

export interface TunnelListResponse {
  tunnels: TunnelListItem[];
  cur_page_number: number;
  total_count: number;
}

export interface TunnelRemoveReq {
  id: number;
}

export interface TunnelAddReq {
  source: string;
  endpoint: string;
  enabled: number;
  sender: number;
  receiver: number;
  description: string;
  tunnel_type: number;
  password: string;
  username: string;
  is_compressed: number;
  encryption_method: string;
  custom_mapping: Record<string, string>;
}

export interface TunnelUpdateReq extends TunnelAddReq {
  id: number;
}

// Tunnel type enum
export enum TunnelType {
  TCP = 0,
  UDP = 1,
  SOCKS5 = 2,
  HTTP = 3,
  Shadowsocks = 4,
}

export const TunnelTypeLabels: Record<number, string> = {
  [TunnelType.TCP]: 'tcp',
  [TunnelType.UDP]: 'udp',
  [TunnelType.SOCKS5]: 'socks5',
  [TunnelType.HTTP]: 'http',
  [TunnelType.Shadowsocks]: 'shadowsocks',
};

export const tunnelTransportEncryptionOptions = [
  { value: 'None', labelKey: 'none' },
  { value: 'Aes128', labelKey: 'aes128' },
  { value: 'Xor', labelKey: 'xor' },
];

export const tunnelShadowsocksMethodOptions = [
  { value: 'chacha20-ietf-poly1305', label: 'chacha20-ietf-poly1305' },
  { value: 'aes-128-gcm', label: 'aes-128-gcm' },
  { value: 'aes-256-gcm', label: 'aes-256-gcm' },
];

export const clientBuildTargetOptions = [
  { value: 'windows-amd64', label: { zh: 'Windows x64', en: 'Windows x64' } },
  { value: 'windows-arm64', label: { zh: 'Windows ARM64', en: 'Windows ARM64' } },
  { value: 'linux-amd64', label: { zh: 'Linux x64', en: 'Linux x64' } },
  { value: 'linux-arm64', label: { zh: 'Linux ARM64', en: 'Linux ARM64' } },
  { value: 'linux-armv7', label: { zh: 'Linux ARMv7 (armv7l)', en: 'Linux ARMv7 (armv7l)' } },
];

export const defaultShadowsocksMethod = 'chacha20-ietf-poly1305';

export const playerKeyCharacters = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz23456789';
export const playerKeyLength = 20;

export const ssCipherMethods = [
  'chacha20-ietf-poly1305',
  'aes-128-gcm',
  'aes-256-gcm',
];

export function isDynamicTargetTunnelType(tunnelType: number): boolean {
  return tunnelType === 2 || tunnelType === 3 || tunnelType === 4;
}

export function isUsernameTunnelType(tunnelType: number): boolean {
  return tunnelType === 2 || tunnelType === 3;
}

export function isPasswordTunnelType(tunnelType: number): boolean {
  return tunnelType === 2 || tunnelType === 3 || tunnelType === 4;
}

export function isShadowsocksTunnelType(tunnelType: number): boolean {
  return tunnelType === 4;
}
