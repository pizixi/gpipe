import React, { useState, useRef, useEffect, useCallback } from 'react';
import { useTranslation } from 'react-i18next';
import { usePlayerStore } from '../stores/playerStore';
import type { PlayerListItem } from '../types';

interface Props {
  value: number;
  onChange: (value: number) => void;
}

const PlayerCombobox: React.FC<Props> = ({ value, onChange }) => {
  const { t } = useTranslation();
  const players = usePlayerStore((s) => s.players);
  const [open, setOpen] = useState(false);
  const [search, setSearch] = useState('');
  const [highlightIndex, setHighlightIndex] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);
  const searchInputRef = useRef<HTMLInputElement>(null);

  const getDisplayLabel = useCallback(
    (id: number) => {
      if (id === 0) return t('server_side');
      const p = players.find((pl) => pl.id === id);
      if (!p) return String(id);
      return p.remark ? `${p.id} · ${p.remark}` : String(p.id);
    },
    [players, t],
  );

  const filtered = players
    .filter((p) => {
      if (!search.trim()) return true;
      const kw = search.trim().toLowerCase();
      return String(p.id).includes(kw) || (p.remark ?? '').toLowerCase().includes(kw);
    })
    .sort((a, b) => a.id - b.id);

  // Build options: selected first, then server(0), then rest
  const buildOptions = (): Array<{ id: number; player?: PlayerListItem; isServer?: boolean }> => {
    const opts: Array<{ id: number; player?: PlayerListItem; isServer?: boolean }> = [];
    const selectedPlayer = filtered.find((p) => p.id === value);
    if (selectedPlayer) opts.push({ id: selectedPlayer.id, player: selectedPlayer });
    opts.push({ id: 0, isServer: true });
    filtered
      .filter((p) => p.id !== value)
      .forEach((p) => opts.push({ id: p.id, player: p }));
    return opts;
  };

  const options = open ? buildOptions() : [];

  const selectValue = (v: number) => {
    onChange(v);
    setOpen(false);
    setSearch('');
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (!open) {
      if (e.key === 'ArrowDown' || e.key === 'Enter') {
        e.preventDefault();
        setOpen(true);
      }
      return;
    }
    switch (e.key) {
      case 'ArrowDown':
        e.preventDefault();
        setHighlightIndex((i) => (i + 1) % options.length);
        break;
      case 'ArrowUp':
        e.preventDefault();
        setHighlightIndex((i) => (i - 1 + options.length) % options.length);
        break;
      case 'Enter':
        e.preventDefault();
        if (options[highlightIndex]) {
          selectValue(options[highlightIndex].id);
        }
        break;
      case 'Escape':
        e.preventDefault();
        setOpen(false);
        setSearch('');
        break;
    }
  };

  useEffect(() => {
    if (open && searchInputRef.current) {
      searchInputRef.current.focus();
    }
  }, [open]);

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (containerRef.current && !containerRef.current.contains(e.target as Node)) {
        setOpen(false);
        setSearch('');
      }
    };
    document.addEventListener('mousedown', handler);
    return () => document.removeEventListener('mousedown', handler);
  }, []);

  useEffect(() => {
    setHighlightIndex(0);
  }, [search]);

  return (
    <div ref={containerRef} style={{ position: 'relative', width: '100%' }}>
      <div
        onClick={() => setOpen(!open)}
        style={{
          border: '1px solid #d9d9d9',
          borderRadius: 6,
          padding: '4px 8px',
          cursor: 'pointer',
          background: '#fff',
          minHeight: 32,
          display: 'flex',
          alignItems: 'center',
        }}
      >
        {getDisplayLabel(value)}
      </div>
      {open && (
        <div
          style={{
            position: 'absolute',
            top: '100%',
            left: 0,
            right: 0,
            zIndex: 1000,
            background: '#fff',
            border: '1px solid #d9d9d9',
            borderRadius: 6,
            boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
            maxHeight: 300,
            overflow: 'auto',
          }}
          onKeyDown={handleKeyDown}
        >
          <div style={{ padding: 4 }}>
            <input
              ref={searchInputRef}
              type="text"
              value={search}
              onChange={(e) => setSearch(e.target.value)}
              onKeyDown={handleKeyDown}
              placeholder={t('tunnel_player_picker_placeholder')}
              style={{
                width: '100%',
                border: '1px solid #d9d9d9',
                borderRadius: 4,
                padding: '4px 8px',
                outline: 'none',
                boxSizing: 'border-box',
              }}
            />
          </div>
          <div>
            {options.map((opt, idx) => (
              <div
                key={opt.isServer ? 'server-0' : opt.id}
                onClick={() => selectValue(opt.id)}
                onMouseEnter={() => setHighlightIndex(idx)}
                style={{
                  padding: '6px 12px',
                  cursor: 'pointer',
                  display: 'flex',
                  justifyContent: 'space-between',
                  alignItems: 'center',
                  background: idx === highlightIndex ? '#f0f0f0' : undefined,
                  fontWeight: opt.id === value ? 600 : undefined,
                }}
              >
                <span>
                  {opt.isServer ? (
                    <>
                      0 · {t('server_side')}{' '}
                      <span
                        style={{
                          fontSize: 11,
                          background: '#e6f7ff',
                          padding: '0 4px',
                          borderRadius: 4,
                        }}
                      >
                        {t('tunnel_player_picker_default_tag')}
                      </span>
                    </>
                  ) : (
                    <>
                      {opt.player!.id}
                      {opt.player!.remark ? ` · ${opt.player!.remark}` : ''}
                    </>
                  )}
                </span>
                {opt.player && !opt.isServer && (
                  <span
                    style={{
                      width: 8,
                      height: 8,
                      borderRadius: '50%',
                      background: opt.player.online ? '#52c41a' : '#d9d9d9',
                      flexShrink: 0,
                    }}
                    title={t(opt.player.online ? 'online' : 'offline')}
                  />
                )}
              </div>
            ))}
            {options.length === 0 && (
              <div style={{ padding: '8px 12px', color: '#999' }}>
                {t('tunnel_player_picker_empty')}
              </div>
            )}
            <div
              style={{
                padding: '4px 12px',
                fontSize: 11,
                color: '#999',
                borderTop: '1px solid #f0f0f0',
              }}
            >
              {t('tunnel_player_picker_manual_hint')}
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

export default PlayerCombobox;
