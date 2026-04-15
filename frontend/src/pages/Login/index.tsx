import React, { useState } from 'react';
import { Card, Form, Input, Button, Typography, message } from 'antd';
import { UserOutlined, LockOutlined, TeamOutlined, NodeIndexOutlined, CloudDownloadOutlined } from '@ant-design/icons';
import { useTranslation } from 'react-i18next';
import { useAuthStore } from '../../stores/authStore';
import LanguageSwitcher from '../../components/LanguageSwitcher';
import './login.css';

const { Title, Text, Paragraph } = Typography;

const LoginPage: React.FC = () => {
  const { t } = useTranslation();
  const login = useAuthStore((s) => s.login);
  const [loading, setLoading] = useState(false);

  const onFinish = async (values: { username: string; password: string }) => {
    setLoading(true);
    const result = await login(values.username, values.password);
    setLoading(false);
    if (!result.success) {
      message.error(t('login_failed') + (result.msg || ''));
    }
  };

  const features = [
    { icon: <TeamOutlined />, title: t('login_feature_players') },
    { icon: <NodeIndexOutlined />, title: t('login_feature_tunnels') },
    { icon: <CloudDownloadOutlined />, title: t('login_feature_clients') },
  ];

  return (
    <div className="login-shell">
      <div className="login-frame">
        <section className="login-hero">
          <div className="login-hero-inner">
            <span className="login-eyebrow">GPipe</span>
            <Title className="login-hero-title" level={1}>
              {t('login_intro_title')}
            </Title>
            <Paragraph className="login-hero-text">
              {t('login_intro_text')}
            </Paragraph>
            <div className="login-feature-list">
              {features.map((feature) => (
                <div key={feature.title} className="login-feature-chip">
                  {feature.icon}
                  <strong>{feature.title}</strong>
                </div>
              ))}
            </div>
          </div>
        </section>

        <Card className="login-card">
          <div className="login-card-head">
            <Title className="login-card-title" level={3}>
              {t('login_title')}
            </Title>
            <Text className="login-card-subtitle">
              {t('login_subtitle')}
            </Text>
          </div>

          <Form className="login-form" onFinish={onFinish} layout="vertical" size="large">
            <Form.Item name="username" rules={[{ required: true }]}>
              <Input
                prefix={<UserOutlined />}
                placeholder={t('username_placeholder')}
                autoFocus
              />
            </Form.Item>
            <Form.Item name="password" rules={[{ required: true }]}>
              <Input.Password
                prefix={<LockOutlined />}
                placeholder={t('password_placeholder')}
              />
            </Form.Item>
            <Form.Item style={{ marginBottom: 14 }}>
              <Button className="login-submit" type="primary" htmlType="submit" block loading={loading}>
                {t('login_button')}
              </Button>
            </Form.Item>
          </Form>

          <div className="login-footer">
            <LanguageSwitcher />
          </div>
        </Card>
      </div>
    </div>
  );
};

export default LoginPage;
