import React, { useEffect, useState } from 'react';
import { Table, Button, Input, Modal, message, Tooltip, Select } from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  PlayCircleOutlined,
  PauseCircleOutlined,
  SearchOutlined,
  SwapRightOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import { useTranslation } from 'react-i18next';
import { useTunnelStore } from '../../stores/tunnelStore';
import { usePlayerStore } from '../../stores/playerStore';
import type { TunnelListItem } from '../../types';
import { TunnelTypeLabels } from '../../types';
import { useElementHeight } from '../../hooks/useElementHeight';
import * as tunnelsApi from '../../api/tunnels';
import AddTunnelModal from '../../modals/AddTunnelModal';
import EditTunnelModal from '../../modals/EditTunnelModal';
import StatusPill from '../../components/StatusPill';

const protocolStyles: Record<number, { color: string; background: string }> = {
  0: { color: '#0f766e', background: '#ecfdf5' },
  1: { color: '#2563eb', background: '#eff6ff' },
  2: { color: '#7c3aed', background: '#f5f3ff' },
  3: { color: '#c2410c', background: '#fff7ed' },
  4: { color: '#b45309', background: '#fffbeb' },
};

const tableBodyOffset = 56;

interface Props {
  selectedPlayerId: number | null;
  onSelectedPlayerIdChange: (playerId: number | null) => void;
}

const ProtocolIcon: React.FC<{ type: number }> = ({ type }) => {
  switch (type) {
    case 0:
      return (
        <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <rect x="1.6" y="4.6" width="3.6" height="6.8" rx="1.2" stroke="currentColor" strokeWidth="1.3" />
          <rect x="10.8" y="4.6" width="3.6" height="6.8" rx="1.2" stroke="currentColor" strokeWidth="1.3" />
          <path d="M5.9 8h4.7" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
          <path d="M8.7 6.3 10.4 8 8.7 9.7" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
    case 1:
      return (
        <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <circle cx="3" cy="8" r="1.5" fill="currentColor" />
          <circle cx="12.5" cy="4.5" r="1.4" fill="currentColor" />
          <circle cx="12.5" cy="11.5" r="1.4" fill="currentColor" />
          <path d="M4.8 8C7 8 9.1 6.5 10.9 4.9M4.8 8C7 8 9.1 9.5 10.9 11.1" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
        </svg>
      );
    case 2:
      return (
        <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <circle cx="4" cy="4" r="1.6" stroke="currentColor" strokeWidth="1.3" />
          <circle cx="12" cy="4" r="1.6" stroke="currentColor" strokeWidth="1.3" />
          <circle cx="8" cy="12" r="1.6" stroke="currentColor" strokeWidth="1.3" />
          <path d="M5.3 4.8 6.9 10.6M10.7 4.8 9.1 10.6M5.8 4H10.2" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
        </svg>
      );
    case 3:
      return (
        <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M2.2 4.5A1.5 1.5 0 0 1 3.7 3h8.6a1.5 1.5 0 0 1 1.5 1.5v7a1.5 1.5 0 0 1-1.5 1.5H3.7a1.5 1.5 0 0 1-1.5-1.5v-7Z" stroke="currentColor" strokeWidth="1.3" />
          <path d="M2.5 6.2h11M5.8 9.8 4.4 8.4l1.4-1.4M10.2 7l1.4 1.4-1.4 1.4" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
    case 4:
      return (
        <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <path d="M8 2.1 12.6 3.8v3.7c0 3-1.8 5.2-4.6 6.4-2.8-1.2-4.6-3.4-4.6-6.4V3.8L8 2.1Z" stroke="currentColor" strokeWidth="1.3" strokeLinejoin="round" />
          <path d="M6.2 8 7.4 9.2 9.9 6.7" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" strokeLinejoin="round" />
        </svg>
      );
    default:
      return (
        <svg viewBox="0 0 16 16" fill="none" aria-hidden="true">
          <circle cx="8" cy="8" r="5.5" stroke="currentColor" strokeWidth="1.3" />
          <path d="M8 5.1v3.3M8 11h.01" stroke="currentColor" strokeWidth="1.3" strokeLinecap="round" />
        </svg>
      );
  }
};

const TunnelsTab: React.FC<Props> = ({ selectedPlayerId, onSelectedPlayerIdChange }) => {
  const { t } = useTranslation();
  const { tunnels, loadTunnels } = useTunnelStore();
  const players = usePlayerStore((s) => s.players);
  const [search, setSearch] = useState('');
  const [protocolFilter, setProtocolFilter] = useState<number | undefined>(undefined);
  const [statusFilter, setStatusFilter] = useState<boolean | undefined>(undefined);
  const [addOpen, setAddOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [editingTunnel, setEditingTunnel] = useState<TunnelListItem | null>(null);
  const [tableRegionRef, tableRegionHeight] = useElementHeight<HTMLDivElement>();

  useEffect(() => {
    loadTunnels();
  }, [loadTunnels]);

  const getPlayerLabel = (id: number) => {
    if (id === 0) return t('server_side');
    const player = players.find((item) => item.id === id);
    if (!player) return String(id);
    return player.remark ? `${player.id} · ${player.remark}` : String(player.id);
  };

  const playerOptions = [
    { value: 0, label: `0 · ${t('server_side')}` },
    ...players
      .slice()
      .sort((a, b) => a.id - b.id)
      .map((player) => ({
        value: player.id,
        label: player.remark ? `${player.id} · ${player.remark}` : String(player.id),
      })),
  ];

  const protocolOptions = Object.entries(TunnelTypeLabels).map(([value, labelKey]) => ({
    value: Number(value),
    label: t(labelKey),
  }));

  const statusOptions = [
    { value: 'enabled', label: t('enabled') },
    { value: 'disabled', label: t('disabled') },
  ];

  const filtered = tunnels.filter((tun) => {
    if (selectedPlayerId !== null && tun.sender !== selectedPlayerId && tun.receiver !== selectedPlayerId) {
      return false;
    }
    if (protocolFilter !== undefined && tun.tunnel_type !== protocolFilter) {
      return false;
    }
    if (statusFilter !== undefined && tun.enabled !== statusFilter) {
      return false;
    }
    if (!search.trim()) {
      return true;
    }

    const keyword = search.trim().toLowerCase();
    const searchFields = [
      String(tun.id),
      tun.source,
      tun.endpoint,
      tun.description,
      tun.username,
      tun.encryption_method,
      getPlayerLabel(tun.sender),
      getPlayerLabel(tun.receiver),
      t(TunnelTypeLabels[tun.tunnel_type] || 'not_set'),
    ];

    return searchFields.some((field) => String(field ?? '').toLowerCase().includes(keyword));
  });

  const handleDelete = (id: number) => {
    Modal.confirm({
      title: t('confirm_delete_tunnel'),
      onOk: async () => {
        const res = await tunnelsApi.removeTunnel({ id });
        if (res.code === 0) loadTunnels();
        else message.error(t('delete_tunnel_failed') + res.msg);
      },
    });
  };

  const handleToggle = async (tunnel: TunnelListItem) => {
    const freshData = await tunnelsApi.fetchTunnels();
    const fresh = (freshData.tunnels || []).find((item) => item.id === tunnel.id);
    if (!fresh) {
      message.error(t('tunnel_not_found'));
      return;
    }
    const res = await tunnelsApi.updateTunnel({
      id: fresh.id,
      source: fresh.source,
      endpoint: fresh.endpoint,
      enabled: fresh.enabled ? 0 : 1,
      sender: fresh.sender,
      receiver: fresh.receiver,
      description: fresh.description,
      tunnel_type: fresh.tunnel_type,
      password: fresh.password,
      username: fresh.username,
      is_compressed: fresh.is_compressed ? 1 : 0,
      encryption_method: fresh.encryption_method,
      custom_mapping: fresh.custom_mapping || {},
    });
    if (res.code === 0) loadTunnels();
    else message.error(t('update_tunnel_failed') + res.msg);
  };

  const handleResetFilters = () => {
    setSearch('');
    setProtocolFilter(undefined);
    setStatusFilter(undefined);
    onSelectedPlayerIdChange(null);
  };

  const tableScrollY = tableRegionHeight > tableBodyOffset ? tableRegionHeight - tableBodyOffset : undefined;

  const columns: ColumnsType<TunnelListItem> = [
    {
      title: t('receiver_id'),
      dataIndex: 'receiver',
      sorter: (a, b) => a.receiver - b.receiver,
      width: 240,
      render: (value: number) => {
        const label = getPlayerLabel(value);
        return (
          <Tooltip title={label}>
            <span className="table-chip">{label}</span>
          </Tooltip>
        );
      },
    },
    {
      title: t('source'),
      dataIndex: 'source',
      sorter: (a, b) => (a.source ?? '').localeCompare(b.source ?? ''),
      ellipsis: true,
      width: 178,
      render: (value: string) => (
        <span style={{ fontFamily: 'monospace', fontSize: 14, whiteSpace: 'nowrap' }}>
          {value || <span style={{ color: '#bfbfbf' }}>-</span>}
        </span>
      ),
    },
    {
      title: '',
      width: 44,
      align: 'center' as const,
      render: () => <SwapRightOutlined style={{ color: '#94a3b8', fontSize: 18 }} />,
    },
    {
      title: t('sender_id'),
      dataIndex: 'sender',
      sorter: (a, b) => a.sender - b.sender,
      width: 240,
      render: (value: number) => {
        const label = getPlayerLabel(value);
        return (
          <Tooltip title={label}>
            <span className="table-chip">{label}</span>
          </Tooltip>
        );
      },
    },
    {
      title: t('target_tab_title'),
      dataIndex: 'endpoint',
      ellipsis: true,
      width: 178,
      render: (value: string) => (
        <span style={{ fontFamily: 'monospace', fontSize: 14, whiteSpace: 'nowrap' }}>
          {value || <span style={{ color: '#bfbfbf' }}>-</span>}
        </span>
      ),
    },
    {
      title: t('protocol_type'),
      dataIndex: 'tunnel_type',
      sorter: (a, b) => a.tunnel_type - b.tunnel_type,
      width: 180,
      render: (value: number) => {
        const label = t(TunnelTypeLabels[value] || 'not_set');
        const style = protocolStyles[value] || { color: '#475569', background: '#f8fafc' };
        return (
          <Tooltip title={label}>
            <span className="protocol-badge" style={style}>
              <ProtocolIcon type={value} />
              <span>{label}</span>
            </span>
          </Tooltip>
        );
      },
    },
    {
      title: t('status'),
      dataIndex: 'enabled',
      sorter: (a, b) => Number(a.enabled) - Number(b.enabled),
      width: 132,
      align: 'center' as const,
      render: (value: boolean) => (
        <StatusPill variant={value ? 'enabled' : 'disabled'} label={t(value ? 'enabled' : 'disabled')} />
      ),
    },
    {
      title: t('description'),
      dataIndex: 'description',
      ellipsis: true,
      render: (value: string) => {
        if (!value) {
          return <span style={{ color: '#bfbfbf' }}>-</span>;
        }
        return (
          <Tooltip title={value}>
            <span>{value}</span>
          </Tooltip>
        );
      },
    },
    {
      title: t('actions'),
      width: 154,
      fixed: 'right' as const,
      render: (_, record) => (
        <div className="table-action-group">
          <Tooltip title={t('edit_tunnel')}>
            <Button
              className="table-action-button"
              type="text"
              icon={<EditOutlined />}
              onClick={() => {
                setEditingTunnel(record);
                setEditOpen(true);
              }}
            />
          </Tooltip>
          <Tooltip title={t('delete_button')}>
            <Button className="table-action-button" type="text" danger icon={<DeleteOutlined />} onClick={() => handleDelete(record.id)} />
          </Tooltip>
          <Tooltip title={t(record.enabled ? 'disabled' : 'enabled')}>
            <Button
              className="table-action-button"
              type="text"
              icon={
                record.enabled ? (
                  <PauseCircleOutlined style={{ color: '#f59e0b' }} />
                ) : (
                  <PlayCircleOutlined style={{ color: '#16a34a' }} />
                )
              }
              onClick={() => handleToggle(record)}
            />
          </Tooltip>
        </div>
      ),
    },
  ];

  return (
    <div className="dashboard-list-page">
      <div className="dashboard-list-toolbar">
        <div className="dashboard-list-filters">
          <Input
            prefix={<SearchOutlined />}
            placeholder={t('search_tunnel_placeholder')}
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            style={{ width: 320 }}
            allowClear
          />
          <Select
            showSearch
            allowClear
            optionFilterProp="label"
            placeholder={t('tunnel_filter_player_placeholder')}
            options={playerOptions}
            value={selectedPlayerId ?? undefined}
            onChange={(value) => onSelectedPlayerIdChange(typeof value === 'number' ? value : null)}
            style={{ width: 240 }}
          />
          <Select
            allowClear
            optionFilterProp="label"
            placeholder={t('tunnel_filter_protocol_placeholder')}
            options={protocolOptions}
            value={protocolFilter}
            onChange={(value) => setProtocolFilter(typeof value === 'number' ? value : undefined)}
            style={{ width: 156 }}
          />
          <Select
            allowClear
            optionFilterProp="label"
            placeholder={t('tunnel_filter_status_placeholder')}
            options={statusOptions}
            value={statusFilter === undefined ? undefined : statusFilter ? 'enabled' : 'disabled'}
            onChange={(value) => {
              if (value === 'enabled') setStatusFilter(true);
              else if (value === 'disabled') setStatusFilter(false);
              else setStatusFilter(undefined);
            }}
            style={{ width: 148 }}
          />
          {(search.trim() !== '' || selectedPlayerId !== null || protocolFilter !== undefined || statusFilter !== undefined) && (
            <Button onClick={handleResetFilters}>
              {t('reset_filters')}
            </Button>
          )}
        </div>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>
          {t('add_tunnel')}
        </Button>
      </div>
      <div ref={tableRegionRef} className="dashboard-table-region">
        <Table<TunnelListItem>
          className="dashboard-data-table"
          columns={columns}
          dataSource={filtered}
          rowKey="id"
          size="middle"
          pagination={false}
          scroll={{ x: 1348, y: tableScrollY }}
          locale={{ emptyText: t('empty_tunnels') }}
          bordered
          tableLayout="fixed"
          style={{ borderRadius: 8, overflow: 'hidden' }}
        />
      </div>
      <AddTunnelModal
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onSuccess={loadTunnels}
      />
      <EditTunnelModal
        open={editOpen}
        tunnel={editingTunnel}
        onClose={() => {
          setEditOpen(false);
          setEditingTunnel(null);
        }}
        onSuccess={loadTunnels}
      />
    </div>
  );
};

export default TunnelsTab;
