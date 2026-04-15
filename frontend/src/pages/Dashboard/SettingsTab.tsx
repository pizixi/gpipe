import React, { useEffect, useState } from 'react';
import { Form, Input, Switch, Select, Button, Card, Typography, message } from 'antd';
import { SaveOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { ClientBuildSettingsPayload } from '../../types';
import { ssCipherMethods, defaultShadowsocksMethod } from '../../types';
import * as settingsApi from '../../api/settings';

const { Title, Text } = Typography;

const SettingsTab: React.FC = () => {
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [useSS, setUseSS] = useState(false);

  const loadSettings = async () => {
    try {
      const data = await settingsApi.fetchClientBuildSettings();
      const s = data.settings || {
        server: '',
        enable_tls: false,
        tls_server_name: '',
        use_shadowsocks: false,
        ss_server: '',
        ss_method: defaultShadowsocksMethod,
        ss_password: '',
      };
      form.setFieldsValue(s);
      setUseSS(!!s.use_shadowsocks);
    } catch {
      // ignore
    }
  };

  useEffect(() => {
    loadSettings();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const handleSave = async () => {
    setLoading(true);
    try {
      const values: ClientBuildSettingsPayload = form.getFieldsValue();
      const res = await settingsApi.updateClientBuildSettings(values);
      if (res.code === 0) {
        message.success(t('settings_saved'));
      } else {
        message.error(t('save_settings_failed') + res.msg);
      }
    } catch {
      // ignore
    }
    setLoading(false);
  };

  return (
    <Card className="dashboard-settings-card">
      <Title level={4} style={{ marginTop: 0 }}>{t('client_settings_title')}</Title>
      <Text type="secondary">{t('client_settings_subtitle')}</Text>
      <Form
        form={form}
        className="dashboard-settings-form"
        layout="horizontal"
        labelAlign="left"
        labelWrap
        labelCol={{ flex: '160px' }}
        wrapperCol={{ flex: '1 1 0' }}
        style={{ marginTop: 24 }}
        onFinish={handleSave}
      >
        <Form.Item label={t('server_address')} name="server">
          <Input placeholder={t('server_address_placeholder')} />
        </Form.Item>
        <Form.Item label={t('enable_tls')} name="enable_tls" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item label={t('tls_server_name')} name="tls_server_name">
          <Input placeholder={t('tls_server_name_placeholder')} />
        </Form.Item>
        <Form.Item label={t('use_shadowsocks')} name="use_shadowsocks" valuePropName="checked">
          <Switch onChange={(value) => setUseSS(value)} />
        </Form.Item>
        {useSS && (
          <>
            <Form.Item label={t('shadowsocks_server')} name="ss_server">
              <Input placeholder={t('shadowsocks_server_placeholder')} />
            </Form.Item>
            <Form.Item label={t('shadowsocks_method')} name="ss_method">
              <Select>
                {ssCipherMethods.map((method) => (
                  <Select.Option key={method} value={method}>{method}</Select.Option>
                ))}
              </Select>
            </Form.Item>
            <Form.Item label={t('shadowsocks_password')} name="ss_password">
              <Input.Password placeholder={t('shadowsocks_password_placeholder')} />
            </Form.Item>
          </>
        )}
        <div className="dashboard-settings-actions">
          <Button type="primary" htmlType="submit" icon={<SaveOutlined />} loading={loading}>
            {t('save_settings')}
          </Button>
        </div>
      </Form>
    </Card>
  );
};

export default SettingsTab;
