import React, { useEffect, useState } from 'react';
import { Modal, Form, Input, Button, Space, Alert, message } from 'antd';
import { ThunderboltOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import type { PlayerListItem } from '../types';
import { generateRandomPlayerKey } from '../utils/helpers';
import * as playersApi from '../api/players';

interface Props {
  open: boolean;
  player: PlayerListItem | null;
  onClose: () => void;
  onSuccess: () => void;
}

const EditPlayerModal: React.FC<Props> = ({ open, player, onClose, onSuccess }) => {
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);
  const isOnline = !!player?.online;

  useEffect(() => {
    if (open && player) {
      form.setFieldsValue({ remark: player.remark, key: player.key });
    }
  }, [open, player, form]);

  const handleOk = async () => {
    if (!player) return;
    const values = form.getFieldsValue();
    const newKey = (values.key || '').trim();
    if (isOnline && newKey !== (player.key || '').trim()) {
      message.warning(t('online_player_key_locked'));
      return;
    }
    setLoading(true);
    try {
      const res = await playersApi.updatePlayer({
        id: player.id,
        remark: (values.remark || '').trim(),
        key: newKey,
      });
      if (res.code === 0) {
        onClose();
        onSuccess();
      } else {
        message.error(t('update_player_failed') + res.msg);
      }
    } catch {
      // ignore
    }
    setLoading(false);
  };

  const handleGenKey = () => {
    if (!isOnline) {
      form.setFieldsValue({ key: generateRandomPlayerKey() });
    }
  };

  return (
    <Modal
      title={t('edit_player')}
      open={open}
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>{t('cancel_button')}</Button>,
        <Button key="ok" type="primary" loading={loading} onClick={handleOk}>{t('update_button')}</Button>,
      ]}
      destroyOnClose
    >
      {isOnline && (
        <Alert
          type="warning"
          message={t('online_player_key_locked_hint')}
          style={{ marginBottom: 16 }}
          showIcon
        />
      )}
      <Form form={form} layout="vertical">
        <Form.Item label={t('player_remark')} name="remark">
          <Input placeholder={t('enter_remark_placeholder')} />
        </Form.Item>
        <Form.Item label={t('player_key')}>
          <Space.Compact style={{ width: '100%' }}>
            <Form.Item name="key" noStyle>
              <Input
                placeholder={t('enter_new_key_placeholder')}
                readOnly={isOnline}
                style={{ flex: 1 }}
              />
            </Form.Item>
            <Button
              icon={<ThunderboltOutlined />}
              onClick={handleGenKey}
              disabled={isOnline}
            >
              {t('generate_key_button')}
            </Button>
          </Space.Compact>
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default EditPlayerModal;
