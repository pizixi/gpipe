import React, { useEffect, useRef, useState, useCallback } from 'react';
import { Table, Button, Input, Modal, message, Tooltip } from 'antd';
import {
  PlusOutlined,
  EditOutlined,
  DeleteOutlined,
  DownloadOutlined,
  NodeIndexOutlined,
  SearchOutlined,
} from '@ant-design/icons';
import type { ColumnsType } from 'antd/es/table';
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
    return String(player.id).includes(keyword) || (player.remark ?? '').toLowerCase().includes(keyword);
  });

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

  const tableScrollY = tableRegionHeight > tableBodyOffset ? tableRegionHeight - tableBodyOffset : undefined;

  const columns: ColumnsType<PlayerListItem> = [
    {
      title: t('id'),
      dataIndex: 'id',
      sorter: (a, b) => a.id - b.id,
      width: 112,
      render: (value: number) => (
        <span style={{ fontFamily: 'monospace', fontSize: 14, whiteSpace: 'nowrap', fontVariantNumeric: 'tabular-nums' }}>
          {value}
        </span>
      ),
    },
    {
      title: t('player_remark'),
      dataIndex: 'remark',
      sorter: (a, b) => (a.remark ?? '').localeCompare(b.remark ?? '', 'zh-CN'),
      ellipsis: true,
      render: (value: string) => value || <span style={{ color: '#999' }}>{t('not_set')}</span>,
    },
    {
      title: t('player_key'),
      dataIndex: 'key',
      ellipsis: true,
      render: (value: string) => <span style={{ fontFamily: 'monospace', fontSize: 14 }}>{value}</span>,
    },
    {
      title: t('create_time'),
      dataIndex: 'create_time',
      sorter: (a, b) => new Date(a.create_time).getTime() - new Date(b.create_time).getTime(),
      render: (value: string) => formatDateTime(value),
      width: 180,
    },
    {
      title: t('online_status'),
      dataIndex: 'online',
      sorter: (a, b) => Number(a.online) - Number(b.online),
      width: 132,
      align: 'center' as const,
      render: (online: boolean) => (
        <StatusPill variant={online ? 'online' : 'offline'} label={t(online ? 'online' : 'offline')} />
      ),
    },
    {
      title: t('actions'),
      width: 202,
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
          className="dashboard-data-table"
          columns={columns}
          dataSource={filtered}
          rowKey="id"
          size="middle"
          pagination={false}
          scroll={{ x: 1030, y: tableScrollY }}
          locale={{ emptyText: t('empty_players') }}
          bordered
          tableLayout="fixed"
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
