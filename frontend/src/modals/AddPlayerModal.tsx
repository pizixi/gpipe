import React, { useState } from 'react';
import { Modal, Form, Input, Button, Space, message } from 'antd';
import { ThunderboltOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { generateRandomPlayerKey } from '../utils/helpers';
import * as playersApi from '../api/players';

interface Props {
  open: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

const AddPlayerModal: React.FC<Props> = ({ open, onClose, onSuccess }) => {
  const { t } = useTranslation();
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(false);

  const handleOk = async () => {
    const values = form.getFieldsValue();
    setLoading(true);
    try {
      const res = await playersApi.addPlayer({
        remark: (values.remark || '').trim(),
        key: (values.key || '').trim(),
      });
      if (res.code === 0) {
        form.resetFields();
        onClose();
        onSuccess();
      } else {
        message.error(t('add_player_failed') + res.msg);
      }
    } catch {
      // ignore
    }
    setLoading(false);
  };

  const handleGenKey = () => {
    form.setFieldsValue({ key: generateRandomPlayerKey() });
  };

  return (
    <Modal
      title={t('add_player')}
      open={open}
      onCancel={onClose}
      footer={[
        <Button key="cancel" onClick={onClose}>{t('cancel_button')}</Button>,
        <Button key="ok" type="primary" loading={loading} onClick={handleOk}>{t('add_button')}</Button>,
      ]}
      destroyOnClose
    >
      <Form form={form} layout="vertical">
        <Form.Item label={t('player_remark')} name="remark">
          <Input placeholder={t('enter_remark_placeholder')} />
        </Form.Item>
        <Form.Item label={t('player_key')}>
          <Space.Compact style={{ width: '100%' }}>
            <Form.Item name="key" noStyle>
              <Input placeholder={t('enter_key_placeholder')} style={{ flex: 1 }} />
            </Form.Item>
            <Button icon={<ThunderboltOutlined />} onClick={handleGenKey}>
              {t('generate_key_button')}
            </Button>
          </Space.Compact>
        </Form.Item>
      </Form>
    </Modal>
  );
};

export default AddPlayerModal;
