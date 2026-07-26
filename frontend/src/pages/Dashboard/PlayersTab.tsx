import React, { useEffect, useRef, useState, useCallback } from 'react';
import { Table, Button, Input, Modal, message, Tooltip } from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  DownloadOutlined,
  NodeIndexOutlined,
  SearchOutlined,
  CloudUploadOutlined,
  ArrowRightOutlined,
  CheckCircleFilled,
  ExclamationCircleFilled,
  LoadingOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
import axios from 'axios';
import { useTranslation } from 'react-i18next';
import { usePlayerStore } from '../../stores/playerStore';
import type { PlayerListItem } from '../../types';
import { formatDateTime } from '../../utils/helpers';
import { useElementHeight } from '../../hooks/useElementHeight';
import * as playersApi from '../../api/players';
import AddPlayerModal from '../../modals/AddPlayerModal';
import EditPlayerModal from '../../modals/EditPlayerModal';
import GenerateClientModal from '../../modals/GenerateClientModal';
import StatusPill from '../../components/StatusPill';

const tableBodyOffset = 56;
const activeUpgradeStates = new Set(['offered', 'accepted', 'downloading', 'verifying', 'applying']);

const getCreateTimeMs = (value: string) => {
  const time = new Date(value).getTime();
  return Number.isFinite(time) ? time : 0;
};

const getOptionalTimeMs = (value: string | null | undefined) => {
  const time = new Date(value ?? '').getTime();
  return Number.isFinite(time) ? time : 0;
};

const comparePlayersByDefault = (a: PlayerListItem, b: PlayerListItem) => (
  Number(b.online) - Number(a.online)
  || getOptionalTimeMs(b.last_online_time) - getOptionalTimeMs(a.last_online_time)
  || getCreateTimeMs(b.create_time) - getCreateTimeMs(a.create_time)
  || b.id - a.id
);

const requestErrorMessage = (error: unknown) => {
  if (axios.isAxiosError(error)) {
    const responseMessage = (error.response?.data as { msg?: unknown } | undefined)?.msg;
    return typeof responseMessage === 'string' && responseMessage ? responseMessage : error.message;
  }
  return error instanceof Error ? error.message : String(error);
};

interface Props {
  onOpenPlayerTunnels: (playerId: number) => void;
}

const PlayersTab: React.FC<Props> = ({ onOpenPlayerTunnels }) => {
  const { t } = useTranslation();
  const { players, loadPlayers } = usePlayerStore();
  const [search, setSearch] = useState('');
  const [addOpen, setAddOpen] = useState(false);
  const [editOpen, setEditOpen] = useState(false);
  const [genOpen, setGenOpen] = useState(false);
  const [editingPlayer, setEditingPlayer] = useState<PlayerListItem | null>(null);
  const [genPlayerId, setGenPlayerId] = useState<number | null>(null);
  const searchTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  const [tableRegionRef, tableRegionHeight] = useElementHeight<HTMLDivElement>();

  useEffect(() => {
    loadPlayers();
  }, [loadPlayers]);

  const handleSearch = useCallback(
    (value: string) => {
      setSearch(value);
      if (searchTimerRef.current) clearTimeout(searchTimerRef.current);
      searchTimerRef.current = setTimeout(() => {
        // search is client-side, state change triggers re-render
      }, 180);
    },
    [],
  );

  const filtered = players.filter((player) => {
    if (!search.trim()) return true;
    const keyword = search.trim().toLowerCase();
    return (
      String(player.id).includes(keyword)
      || (player.remark ?? '').toLowerCase().includes(keyword)
    );
  });
  const sortedPlayers = filtered.slice().sort(comparePlayersByDefault);

  const handleDelete = async (player: PlayerListItem) => {
    if (player.online) {
      message.warning(t('online_player_delete_forbidden'));
      return;
    }
    Modal.confirm({
      title: t('confirm_delete_player'),
      onOk: async () => {
        const res = await playersApi.removePlayer({ id: player.id });
        if (res.code === 0) {
          loadPlayers();
        } else {
          message.error(t('delete_player_failed') + res.msg);
        }
      },
    });
  };

  const handleEdit = (player: PlayerListItem) => {
    setEditingPlayer(player);
    setEditOpen(true);
  };

  const handleGenerate = (player: PlayerListItem) => {
    setGenPlayerId(player.id);
    setGenOpen(true);
  };

  const handleUpgrade = (player: PlayerListItem) => {
    Modal.confirm({
      title: t('confirm_upgrade_client'),
      content: t('confirm_upgrade_client_desc', { current: player.client_version, latest: player.latest_version }),
      okText: t('upgrade_client'),
      cancelText: t('cancel_button'),
      onOk: async () => {
        try {
          const res = await playersApi.upgradeClient({ player_id: player.id });
          if (res.code === 0) {
            message.success(t('upgrade_started'));
            await loadPlayers();
          } else {
            message.error(t('upgrade_failed') + res.msg);
          }
        } catch (error) {
          message.error(t('upgrade_failed') + requestErrorMessage(error));
        }
      },
    });
  };

  const tableScrollY = tableRegionHeight > tableBodyOffset ? tableRegionHeight - tableBodyOffset : undefined;

  const columns: ColumnsType<PlayerListItem> = [
    {
      title: t('id'),
      dataIndex: 'id',
      sorter: (a, b) => a.id - b.id,
      render: (value: number) => (
        <span style={{ fontFamily: 'monospace', fontSize: 14, whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' }}>
          {value}
        </span>
      ),
    },
    {
      title: t('player_key'),
      dataIndex: 'key',
      render: (value: string) => <span style={{ fontFamily: 'monospace', fontSize: 14 }}>{value}</span>,
    },
    {
      title: t('client_version'),
      dataIndex: 'client_version',
      sorter: (a, b) => (a.client_version || '').localeCompare(b.client_version || ''),
      render: (value: string, record) => {
        if (!value) {
          return <span className="client-version-badge client-version-badge--unknown">{t('unknown_version')}</span>;
        }
        const active = activeUpgradeStates.has(record.upgrade_status);
        const failed = ['failed', 'rolled_back'].includes(record.upgrade_status);
        const reasonKey = record.upgrade_unavailable_reason
          ? `upgrade_reason_${record.upgrade_unavailable_reason}`
          : 'upgrade_unavailable';
        const badgeState = active
          ? 'active'
          : record.can_upgrade
            ? 'upgrade'
            : record.is_latest
              ? 'latest'
              : 'unavailable';
        const tooltip = (
          <div className="client-version-tooltip">
            <div><span>{t('current_client_version')}</span><strong>v{value}</strong></div>
            <div><span>{t('server_target_version')}</span><strong>v{record.latest_version}</strong></div>
            <div><span>{t('client_platform_detail')}</span><strong>{record.client_platform || t('unknown_platform')}</strong></div>
            {record.can_upgrade && !active && <div className="client-version-tooltip-highlight">{t('upgrade_safe_available')}</div>}
            {!record.can_upgrade && !record.is_latest && !active && <div className="client-version-tooltip-warning">{t(reasonKey)}</div>}
            {active && <div className="client-version-tooltip-highlight">{t(`upgrade_status_${record.upgrade_status}`)} · {record.upgrade_progress}%</div>}
            {failed && record.upgrade_error && <div className="client-version-tooltip-error">{record.upgrade_error}</div>}
          </div>
        );
        return (
          <Tooltip title={tooltip} placement="topLeft" mouseEnterDelay={0.15}>
            <span className={`client-version-badge client-version-badge--${badgeState}`}>
              {active && <LoadingOutlined spin />}
              {record.is_latest && !active && <CheckCircleFilled />}
              <span className="client-version-current">v{value}</span>
              {record.can_upgrade && !active && (
                <>
                  <ArrowRightOutlined className="client-version-arrow" />
                  <span className="client-version-target">v{record.latest_version}</span>
                </>
              )}
              {active && <span className="client-version-progress">{record.upgrade_progress}%</span>}
              {failed && !active && <ExclamationCircleFilled className="client-version-error-icon" />}
            </span>
          </Tooltip>
        );
      },
    },
    {
      title: t('create_time'),
      dataIndex: 'create_time',
      sorter: (a, b) => new Date(a.create_time).getTime() - new Date(b.create_time).getTime(),
      render: (value: string) => formatDateTime(value),
    },
    {
      title: t('last_online_time'),
      dataIndex: 'last_online_time',
      sorter: (a, b) => getOptionalTimeMs(a.last_online_time) - getOptionalTimeMs(b.last_online_time),
      render: (value: string | null) => {
        const text = formatDateTime(value);
        return text || <span className="player-meta-empty">-</span>;
      },
    },
    {
      title: t('player_remark'),
      dataIndex: 'remark',
      sorter: (a, b) => (a.remark ?? '').localeCompare(b.remark ?? '', 'zh-CN'),
      render: (value: string, record) => {
        if (!value) {
          return <span className="player-remark player-remark--empty">{t('not_set')}</span>;
        }
        return (
          <span className={`player-remark player-remark--${record.online ? 'online' : 'offline'}`}>
            {value}
          </span>
        );
      },
    },
    {
      title: t('online_status'),
      dataIndex: 'online',
      sorter: (a, b) => Number(a.online) - Number(b.online),
      align: 'center' as const,
      render: (online: boolean) => (
        <StatusPill variant={online ? 'online' : 'offline'} label={t(online ? 'online' : 'offline')} />
      ),
    },
    {
      title: t('actions'),
      render: (_, record) => (
        <div className="table-action-group">
          <Tooltip title={t('view_player_tunnels')}>
            <Button
              className="table-action-button"
              type="text"
              icon={<NodeIndexOutlined style={{ color: '#2563eb' }} />}
              onClick={() => onOpenPlayerTunnels(record.id)}
            />
          </Tooltip>
          <Tooltip title={t('edit_player')}>
            <Button className="table-action-button" type="text" icon={<EditOutlined />} onClick={() => handleEdit(record)} />
          </Tooltip>
          <Tooltip title={t('generate_client')}>
            <Button className="table-action-button" type="text" icon={<DownloadOutlined style={{ color: '#0d9488' }} />} onClick={() => handleGenerate(record)} />
          </Tooltip>
          <Tooltip title={record.can_upgrade
            ? t('upgrade_client')
            : t(record.is_latest
              ? 'already_latest'
              : record.upgrade_unavailable_reason
                ? `upgrade_reason_${record.upgrade_unavailable_reason}`
                : 'upgrade_unavailable')}>
            <Button
              className="table-action-button"
              type="text"
              icon={<CloudUploadOutlined style={{ color: record.can_upgrade ? '#7c3aed' : undefined }} />}
              disabled={!record.can_upgrade || activeUpgradeStates.has(record.upgrade_status)}
              onClick={() => handleUpgrade(record)}
            />
          </Tooltip>
          <Tooltip title={record.online ? t('online_player_delete_forbidden') : t('delete_button')}>
            <Button
              className="table-action-button"
              type="text"
              danger
              icon={<DeleteOutlined />}
              disabled={record.online}
              onClick={() => handleDelete(record)}
            />
          </Tooltip>
        </div>
      ),
    },
  ];

  return (
    <div className="dashboard-list-page">
      <div className="dashboard-list-toolbar">
        <Input
          prefix={<SearchOutlined />}
          placeholder={t('search_player_placeholder')}
          value={search}
          onChange={(e) => handleSearch(e.target.value)}
          style={{ width: 260 }}
          allowClear
        />
        <Button type="primary" icon={<PlusOutlined />} onClick={() => setAddOpen(true)}>
          {t('add_player')}
        </Button>
      </div>
      <div ref={tableRegionRef} className="dashboard-table-region">
        <Table<PlayerListItem>
          className="dashboard-data-table player-data-table--auto-width"
          columns={columns}
          dataSource={sortedPlayers}
          rowKey="id"
          size="middle"
          pagination={false}
          scroll={{ x: 'max-content', y: tableScrollY }}
          locale={{ emptyText: t('empty_players') }}
          bordered
          tableLayout="auto"
          style={{ borderRadius: 8, overflow: 'hidden' }}
        />
      </div>
      <AddPlayerModal
        open={addOpen}
        onClose={() => setAddOpen(false)}
        onSuccess={loadPlayers}
      />
      <EditPlayerModal
        open={editOpen}
        player={editingPlayer}
        onClose={() => { setEditOpen(false); setEditingPlayer(null); }}
        onSuccess={loadPlayers}
      />
      <GenerateClientModal
        open={genOpen}
        playerId={genPlayerId}
        onClose={() => { setGenOpen(false); setGenPlayerId(null); }}
      />
    </div>
  );
};

export default PlayersTab;
