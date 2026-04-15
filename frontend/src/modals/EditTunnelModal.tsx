import React, { useEffect, useState } from 'react';
import { Modal, Form, Input, Select, Switch, message, Divider, Row, Col } from 'antd';
import { SwapRightOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { TunnelListItem } from '../types';
import {
  TunnelType,
  isDynamicTargetTunnelType,
  isUsernameTunnelType,
  isPasswordTunnelType,
  isShadowsocksTunnelType,
  tunnelTransportEncryptionOptions,
  tunnelShadowsocksMethodOptions,
  defaultShadowsocksMethod,
} from '../types';
import * as tunnelsApi from '../api/tunnels';
import PlayerCombobox from '../components/PlayerCombobox';
import { normalizeListenAddress, simplifyListenAddress } from '../utils/helpers';

interface Props {
  open: boolean;
  tunnel: TunnelListItem | null;
  onClose: () => void;
  onSuccess: () => void;
}

const EditTunnelModal: React.FC<Props> = ({ open, tunnel, onClose, onSuccess }) => {
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const [tunnelType, setTunnelType] = useState(0);
  const [sender, setSender] = useState(0);
  const [receiver, setReceiver] = useState(0);

  useEffect(() => {
    if (open && tunnel) {
      const tt = tunnel.tunnel_type ?? 0;
      setTunnelType(tt);
      setSender(tunnel.sender ?? 0);
      setReceiver(tunnel.receiver ?? 0);
      form.setFieldsValue({
        source: simplifyListenAddress(tunnel.source),
        endpoint: tunnel.endpoint,
        description: tunnel.description,
        tunnel_type: tt,
        password: tunnel.password,
        username: tunnel.username,
        is_compressed: !!tunnel.is_compressed,
        encryption_method: tunnel.encryption_method || (isShadowsocksTunnelType(tt) ? defaultShadowsocksMethod : 'None'),
      });
    }
  }, [open, tunnel, form]);

  const handleTypeChange = (val: number) => {
    setTunnelType(val);
    form.setFieldsValue({
      encryption_method: isShadowsocksTunnelType(val) ? defaultShadowsocksMethod : 'None',
    });
  };

  const encryptionOptions = isShadowsocksTunnelType(tunnelType)
    ? tunnelShadowsocksMethodOptions.map((o) => ({ value: o.value, label: o.label }))
    : tunnelTransportEncryptionOptions.map((o) => ({ value: o.value, label: t(o.labelKey) }));

  const handleOk = async () => {
    if (!tunnel) return;
    const values = form.getFieldsValue();
    const tt = values.tunnel_type ?? 0;
    if (isShadowsocksTunnelType(tt) && !(values.password || '').trim()) {
      message.warning(t('shadowsocks_password_required'));
      return;
    }
    setLoading(true);
    try {
      const res = await tunnelsApi.updateTunnel({
        id: tunnel.id,
        source: normalizeListenAddress(values.source),
        endpoint: isDynamicTargetTunnelType(tt) ? '' : (values.endpoint || '').trim(),
        enabled: tunnel.enabled ? 1 : 0,
        sender,
        receiver,
        description: (values.description || '').trim(),
        tunnel_type: tt,
        password: isPasswordTunnelType(tt) ? (values.password || '').trim() : '',
        username: isUsernameTunnelType(tt) ? (values.username || '').trim() : '',
        is_compressed: values.is_compressed ? 1 : 0,
        encryption_method: values.encryption_method || (isShadowsocksTunnelType(tt) ? defaultShadowsocksMethod : 'None'),
        custom_mapping: tunnel.custom_mapping || {},
      });
      if (res.code === 0) {
        onClose();
        onSuccess();
      } else {
        message.error(t('update_tunnel_failed') + res.msg);
      }
    } catch {
      // ignore
    }
    setLoading(false);
  };

  return (
    <Modal
      title={t('edit_tunnel')}
      open={open}
      onCancel={onClose}
      onOk={handleOk}
      confirmLoading={loading}
      okText={t('update_button')}
      cancelText={t('cancel_button')}
      destroyOnClose
      width={600}
    >
      <Form form={form} layout="horizontal" labelCol={{ span: 6 }} wrapperCol={{ span: 17 }} style={{ marginTop: 16 }}>
        <Form.Item label={t('protocol_type')} name="tunnel_type">
          <Select onChange={handleTypeChange}>
            <Select.Option value={TunnelType.TCP}>{t('tcp')}</Select.Option>
            <Select.Option value={TunnelType.UDP}>{t('udp')}</Select.Option>
            <Select.Option value={TunnelType.SOCKS5}>{t('socks5')}</Select.Option>
            <Select.Option value={TunnelType.HTTP}>{t('http')}</Select.Option>
            <Select.Option value={TunnelType.Shadowsocks}>{t('shadowsocks')}</Select.Option>
          </Select>
        </Form.Item>

        <Divider dashed style={{ margin: '12px 0' }} />

        <Form.Item label={t('receiver_id')}>
          <PlayerCombobox value={receiver} onChange={setReceiver} />
        </Form.Item>
        <Form.Item label={t('source')} name="source" extra={t('source_short_hint')}>
          <Input placeholder={t('source_placeholder')} style={{ fontFamily: 'monospace' }} />
        </Form.Item>

        <Row justify="center" style={{ margin: '-4px 0 8px' }}>
          <Col><SwapRightOutlined style={{ fontSize: 20, color: '#bfbfbf', transform: 'rotate(90deg)' }} /></Col>
        </Row>

        <Form.Item label={t('sender_id')}>
          <PlayerCombobox value={sender} onChange={setSender} />
        </Form.Item>
        {!isDynamicTargetTunnelType(tunnelType) && (
          <Form.Item label={t('target_tab_title')} name="endpoint">
            <Input placeholder={t('target_placeholder')} style={{ fontFamily: 'monospace' }} />
          </Form.Item>
        )}

        <Divider dashed style={{ margin: '12px 0' }} />

        {isUsernameTunnelType(tunnelType) && (
          <Form.Item label={t('username')} name="username">
            <Input placeholder={t('username_placeholder')} />
          </Form.Item>
        )}
        {isPasswordTunnelType(tunnelType) && (
          <Form.Item label={t('password')} name="password">
            <Input.Password placeholder={t('enter_password_placeholder')} />
          </Form.Item>
        )}
        <Form.Item label={t('encryption_method')} name="encryption_method">
          <Select options={encryptionOptions} />
        </Form.Item>
        <Form.Item label={t('enable_compression')} name="is_compressed" valuePropName="checked">
          <Switch />
        </Form.Item>
        <Form.Item label={t('description')} name="description">
          <Input placeholder={t('description_placeholder')} />
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default EditTunnelModal;
