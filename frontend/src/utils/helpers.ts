import { playerKeyCharacters, playerKeyLength } from '../types';

export function generateRandomPlayerKey(): string {
  const bytes = new Uint8Array(playerKeyLength);
  if (window.crypto?.getRandomValues) {
    window.crypto.getRandomValues(bytes);
  } else {
    for (let i = 0; i < bytes.length; i++) {
      bytes[i] = Math.floor(Math.random() * 256);
    }
  }
  return Array.from(bytes, (byte) => playerKeyCharacters[byte % playerKeyCharacters.length]).join(
    '',
  );
}

export function formatDateTime(value: string | null | undefined): string {
  const text = String(value ?? '').trim();
  if (text === '') return '';
  const date = new Date(text);
  if (Number.isNaN(date.getTime())) return text;
  const pad = (num: number) => String(num).padStart(2, '0');
  return `${date.getFullYear()}-${pad(date.getMonth() + 1)}-${pad(date.getDate())} ${pad(date.getHours())}:${pad(date.getMinutes())}:${pad(date.getSeconds())}`;
}

export function normalizeListenAddress(value: string | null | undefined): string {
  const text = String(value ?? '').trim();
  if (text === '') return '';
  if (/^:\d+$/.test(text)) {
    return `0.0.0.0${text}`;
  }
  return text;
}

export function simplifyListenAddress(value: string | null | undefined): string {
  const text = String(value ?? '').trim();
  const match = /^0\.0\.0\.0(:\d+)$/.exec(text);
  if (match) {
    return match[1];
  }
  return text;
}
