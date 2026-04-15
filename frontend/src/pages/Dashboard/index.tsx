import React, { Suspense, lazy, useState } from 'react';
import { Layout, Tabs, Button, Typography, Space, Badge, Spin } from 'antd';
import {
  LogoutOutlined,
  TeamOutlined,
  NodeIndexOutlined,
  SettingOutlined,
  ApiOutlined,
} from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '../../stores/authStore';
import { usePlayerStore } from '../../stores/playerStore';
import LanguageSwitcher from '../../components/LanguageSwitcher';
import './dashboard.css';

const PlayersTab = lazy(() => import('./PlayersTab'));
const TunnelsTab = lazy(() => import('./TunnelsTab'));
const SettingsTab = lazy(() => import('./SettingsTab'));

const { Header, Content } = Layout;
const { Title, Text } = Typography;

const tabFallback = (
  <div className="dashboard-tab-loading">
    <Spin size="large" />
  </div>
);

const DashboardPage: React.FC = () => {
  const { t } = useTranslation();
  const logout = useAuthStore((s) => s.logout);
  const players = usePlayerStore((s) => s.players);
  const onlineCount = players.filter((p) => p.online).length;
  const [activeKey, setActiveKey] = useState('players');
  const [tunnelPlayerFilterId, setTunnelPlayerFilterId] = useState<number | null>(null);

  const handleOpenPlayerTunnels = (playerId: number) => {
    setTunnelPlayerFilterId(playerId);
    setActiveKey('tunnels');
  };

  const items = [
    {
      key: 'players',
      label: (
        <span>
          <TeamOutlined style={{ marginRight: 6 }} />
          {t('player_management')}
          <Badge
            count={players.length}
            style={{ backgroundColor: '#8c8c8c', marginLeft: 8, boxShadow: 'none' }}
            size="small"
          />
        </span>
      ),
      children: (
        <Suspense fallback={tabFallback}>
          <PlayersTab onOpenPlayerTunnels={handleOpenPlayerTunnels} />
        </Suspense>
      ),
    },
    {
      key: 'tunnels',
      label: (
        <span>
          <NodeIndexOutlined style={{ marginRight: 6 }} />
          {t('tunnel_management')}
        </span>
      ),
      children: (
        <Suspense fallback={tabFallback}>
          <TunnelsTab
            selectedPlayerId={tunnelPlayerFilterId}
            onSelectedPlayerIdChange={setTunnelPlayerFilterId}
          />
        </Suspense>
      ),
    },
    {
      key: 'settings',
      label: (
        <span>
          <SettingOutlined style={{ marginRight: 6 }} />
          {t('client_settings')}
        </span>
      ),
      children: (
        <Suspense fallback={tabFallback}>
          <SettingsTab />
        </Suspense>
      ),
    },
  ];

  return (
    <Layout className="dashboard-shell">
      <Header
        style={{
          background: 'linear-gradient(135deg, #0d9488 0%, #0f766e 100%)',
          display: 'flex',
          justifyContent: 'space-between',
          alignItems: 'center',
          padding: '0 28px',
          minHeight: 70,
          height: 'auto',
          boxShadow: '0 2px 8px rgba(0,0,0,0.15)',
        }}
      >
        <Space align="center" size={14}>
          <ApiOutlined style={{ fontSize: 24, color: '#fff' }} />
          <div className="dashboard-header-copy">
            <Title level={4} style={{ margin: 0, color: '#fff', fontWeight: 700, letterSpacing: 0.4 }}>
              {t('console_title')}
            </Title>
            <Text className="dashboard-header-subtitle">
              {t('console_subtitle')}
            </Text>
          </div>
        </Space>
        <Space size={12} align="center">
          <Badge
            status="processing"
            text={
              <Text style={{ color: 'rgba(255,255,255,0.85)', fontSize: 13 }}>
                {onlineCount} {t('online')}
              </Text>
            }
          />
          <LanguageSwitcher />
          <Button
            type="text"
            icon={<LogoutOutlined />}
            onClick={logout}
            style={{ color: 'rgba(255,255,255,0.85)' }}
          >
            {t('logout_button')}
          </Button>
        </Space>
      </Header>
      <Content className="dashboard-content">
        <Tabs
          className="dashboard-tabs"
          items={items}
          activeKey={activeKey}
          onChange={setActiveKey}
          type="card"
          destroyInactiveTabPane
        />
      </Content>
    </Layout>
  );
};

export default DashboardPage;
