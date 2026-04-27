import React, { useEffect, useState } from 'react';
import { Modal, Select, Typography, Descriptions, Button, message, Form, Input, Switch, Spin } from 'antd';
import { DownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { clientBuildTargetOptions, defaultShadowsocksMethod, ssCipherMethods } from '../types';
import type { ClientBuildSettingsPayload } from '../types';
import { usePlayerStore } from '../stores/playerStore';
import * as settingsApi from '../api/settings';

const { Text, Paragraph } = Typography;
const formItemStyle: React.CSSProperties = { marginBottom: 12 };
const switchItemStyle: React.CSSProperties = { marginBottom: 10 };

interface Props {
  open: boolean;
  playerId: number | null;
  onClose: () => void;
}

const withDefaults = (
  settings?: Partial<ClientBuildSettingsPayload> | null,
): ClientBuildSettingsPayload => ({
  server: settings?.server ?? '',
  enable_tls: !!settings?.enable_tls,
  tls_server_name: settings?.tls_server_name ?? '',
  use_shadowsocks: !!settings?.use_shadowsocks,
  ss_server: settings?.ss_server ?? '',
  ss_method: settings?.ss_method || defaultShadowsocksMethod,
  ss_password: settings?.ss_password ?? '',
});

const normalizeSettings = (settings: ClientBuildSettingsPayload): ClientBuildSettingsPayload => {
  const next = withDefaults(settings);
  if (!next.use_shadowsocks) {
    next.ss_server = '';
    next.ss_method = defaultShadowsocksMethod;
    next.ss_password = '';
  }
  return next;
};

const GenerateClientModal: React.FC<Props> = ({ open, playerId, onClose }) => {
  const { t, i18n } = useTranslation();
  const [form] = Form.useForm<ClientBuildSettingsPayload>();
  const players = usePlayerStore((s) => s.players);
  const [target, setTarget] = useState(clientBuildTargetOptions[0].value);
  const [loading, setLoading] = useState(false);
  const [settingsLoading, setSettingsLoading] = useState(false);
  const [useSS, setUseSS] = useState(false);
  const [customized, setCustomized] = useState(false);

  const player = players.find((p) => p.id === playerId);
  const lang = (i18n.language || 'zh') as 'zh' | 'en';

  useEffect(() => {
    if (!open || !playerId) return;
    setTarget(clientBuildTargetOptions[0].value);
    setCustomized(false);
    setSettingsLoading(true);
    settingsApi.fetchPlayerClientBuildSettings(playerId).then((data) => {
      const nextSettings = withDefaults(data.settings);
      form.setFieldsValue(nextSettings);
      setUseSS(nextSettings.use_shadowsocks);
      setCustomized(!!data.customized);
    }).catch(() => {
      const nextSettings = withDefaults();
      form.setFieldsValue(nextSettings);
      setUseSS(false);
      setCustomized(false);
    }).finally(() => {
      setSettingsLoading(false);
    });
  }, [form, open, playerId]);

  const handleUseShadowsocksChange = (value: boolean) => {
    setUseSS(value);
    if (value && !form.getFieldValue('ss_method')) {
      form.setFieldValue('ss_method', defaultShadowsocksMethod);
    }
  };

  const handleDownload = async () => {
    if (!playerId) return;

    let values: ClientBuildSettingsPayload;
    try {
      values = await form.validateFields();
    } catch {
      return;
    }

    setLoading(true);
    try {
      const result = await settingsApi.generateClient({
        player_id: playerId,
        target,
        settings: normalizeSettings(values),
      });
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
    } finally {
      setLoading(false);
    }
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
          disabled={settingsLoading || !playerId}
          onClick={handleDownload}
        >
          {t('download_client')}
        </Button>,
      ]}
      destroyOnClose
      width={640}
    >
      <Spin spinning={settingsLoading}>
        {player && (
          <Descriptions
            column={2}
            size="small"
            labelStyle={{ color: '#6b7280', fontWeight: 500 }}
            contentStyle={{ fontSize: 14 }}
            style={{
              marginBottom: 14,
              padding: '10px 12px',
              background: '#fafafa',
              border: '1px solid #f0f0f0',
              borderRadius: 6,
            }}
          >
            <Descriptions.Item label={t('id')}>{player.id}</Descriptions.Item>
            <Descriptions.Item label={t('player_remark')}>
              {player.remark || <Text type="secondary">{t('not_set')}</Text>}
            </Descriptions.Item>
          </Descriptions>
        )}

        <Form
          form={form}
          layout="horizontal"
          labelAlign="left"
          labelWrap
          colon={false}
          requiredMark={false}
          labelCol={{ flex: '164px' }}
          wrapperCol={{ flex: '1 1 0' }}
          preserve={false}
          style={{ marginTop: 2 }}
        >
          <Form.Item label={t('client_target')} style={formItemStyle}>
            <Select
              value={target}
              onChange={setTarget}
              style={{ width: '100%' }}
            >
              {clientBuildTargetOptions.map((opt) => (
                <Select.Option key={opt.value} value={opt.value}>
                  {opt.label[lang] || opt.label.zh}
                </Select.Option>
              ))}
            </Select>
          </Form.Item>

          <Form.Item
            label={t('server_address')}
            name="server"
            rules={[{ required: true, message: t('server_address_required') }]}
            style={formItemStyle}
          >
            <Input placeholder={t('server_address_placeholder')} />
          </Form.Item>

          <Form.Item label={t('enable_tls')} name="enable_tls" valuePropName="checked" style={switchItemStyle}>
            <Switch />
          </Form.Item>

          <Form.Item label={t('tls_server_name')} name="tls_server_name" style={formItemStyle}>
            <Input placeholder={t('tls_server_name_placeholder')} />
          </Form.Item>

          <Form.Item
            label={t('use_shadowsocks')}
            name="use_shadowsocks"
            valuePropName="checked"
            style={switchItemStyle}
          >
            <Switch onChange={handleUseShadowsocksChange} />
          </Form.Item>

          {useSS && (
            <>
              <Form.Item
                label={t('shadowsocks_server')}
                name="ss_server"
                rules={[{ required: true, message: t('shadowsocks_server_required') }]}
                style={formItemStyle}
              >
                <Input placeholder={t('shadowsocks_server_placeholder')} />
              </Form.Item>

              <Form.Item label={t('shadowsocks_method')} name="ss_method" style={formItemStyle}>
                <Select>
                  {ssCipherMethods.map((method) => (
                    <Select.Option key={method} value={method}>{method}</Select.Option>
                  ))}
                </Select>
              </Form.Item>

              <Form.Item
                label={t('shadowsocks_password')}
                name="ss_password"
                rules={[{ required: true, message: t('shadowsocks_password_required') }]}
                style={formItemStyle}
              >
                <Input.Password placeholder={t('shadowsocks_password_placeholder')} />
              </Form.Item>
            </>
          )}
        </Form>

        <Paragraph type="secondary" style={{ fontSize: 13, lineHeight: 1.6, marginBottom: 0 }}>
          {customized ? t('player_client_settings_customized_hint') : t('player_client_settings_default_hint')}
        </Paragraph>
      </Spin>
    </Modal>
  );
};

export default GenerateClientModal;
