import React, { useEffect, useState } from 'react';
import { Modal, Select, Typography, Descriptions, Button, message } from 'antd';
import { DownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { clientBuildTargetOptions, defaultShadowsocksMethod } from '../types';
import type { ClientBuildSettingsPayload } from '../types';
import { usePlayerStore } from '../stores/playerStore';
import * as settingsApi from '../api/settings';

const { Text, Paragraph } = Typography;

interface Props {
  open: boolean;
  playerId: number | null;
  onClose: () => void;
}

const GenerateClientModal: React.FC<Props> = ({ open, playerId, onClose }) => {
  const { t, i18n } = useTranslation();
  const players = usePlayerStore((s) => s.players);
  const [target, setTarget] = useState(clientBuildTargetOptions[0].value);
  const [settings, setSettings] = useState<ClientBuildSettingsPayload | null>(null);
  const [loading, setLoading] = useState(false);

  const player = players.find((p) => p.id === playerId);
  const lang = (i18n.language || 'zh') as 'zh' | 'en';

  useEffect(() => {
    if (open) {
      setTarget(clientBuildTargetOptions[0].value);
      settingsApi.fetchClientBuildSettings().then((data) => {
        setSettings(data.settings || null);
      }).catch(() => {
        setSettings(null);
      });
    }
  }, [open]);

  const handleDownload = async () => {
    if (!playerId) return;
    setLoading(true);
    try {
      const result = await settingsApi.generateClient({ player_id: playerId, target });
      if (result.success && result.blob) {
        const url = URL.createObjectURL(result.blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = result.filename || `gpipe-client-${playerId}`;
        document.body.appendChild(link);
        link.click();
        link.remove();
        URL.revokeObjectURL(url);
        onClose();
      } else {
        message.error(t('generate_client_failed') + (result.error || ''));
      }
    } catch {
      // ignore
    }
    setLoading(false);
  };

  return (
    <Modal
      title={t('generate_client')}
      open={open}
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>{t('cancel_button')}</Button>,
        <Button
          key="download"
          type="primary"
          icon={<DownloadOutlined />}
          loading={loading}
          onClick={handleDownload}
        >
          {t('download_client')}
        </Button>,
      ]}
      destroyOnClose
      width={480}
    >
      {player && (
        <Descriptions column={1} size="small" style={{ marginBottom: 16 }}>
          <Descriptions.Item label={t('id')}>{player.id}</Descriptions.Item>
          <Descriptions.Item label={t('player_remark')}>
            {player.remark || <Text type="secondary">{t('not_set')}</Text>}
          </Descriptions.Item>
        </Descriptions>
      )}

      <div style={{ marginBottom: 16 }}>
        <Text strong>{t('client_target')}</Text>
        <Select
          value={target}
          onChange={setTarget}
          style={{ width: '100%', marginTop: 4 }}
        >
          {clientBuildTargetOptions.map((opt) => (
            <Select.Option key={opt.value} value={opt.value}>
              {opt.label[lang] || opt.label.zh}
            </Select.Option>
          ))}
        </Select>
      </div>

      {settings && (
        <div style={{ marginBottom: 16, background: '#fafafa', padding: 12, borderRadius: 6 }}>
          <Text strong style={{ display: 'block', marginBottom: 8 }}>{t('client_settings_title')}</Text>
          <div style={{ fontSize: 13 }}>
            <div>{t('server_address')}: {settings.server || t('not_set')}</div>
            <div>{t('enable_tls')}: {t(settings.enable_tls ? 'enabled' : 'disabled')}</div>
            {settings.tls_server_name && (
              <div>{t('tls_server_name')}: {settings.tls_server_name}</div>
            )}
            <div>{t('use_shadowsocks')}: {t(settings.use_shadowsocks ? 'enabled' : 'disabled')}</div>
            {settings.use_shadowsocks && (
              <>
                <div>{t('shadowsocks_server')}: {settings.ss_server || t('not_set')}</div>
                <div>{t('shadowsocks_method')}: {settings.ss_method || defaultShadowsocksMethod}</div>
              </>
            )}
          </div>
        </div>
      )}

      <Paragraph type="secondary" style={{ fontSize: 12 }}>
        {t('client_download_hint')}
      </Paragraph>
    </Modal>
  );
};

export default GenerateClientModal;
