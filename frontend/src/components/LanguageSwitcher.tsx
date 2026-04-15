import React from 'react';
import { Button, Space } from 'antd';
import { useTranslation } from 'react-i18next';

const LanguageSwitcher: React.FC = () => {
  const { i18n } = useTranslation();
  const current = i18n.language;

  const changeLanguage = (lang: string) => {
    i18n.changeLanguage(lang);
    localStorage.setItem('language', lang);
  };

  return (
    <Space size={4}>
      <Button
        type={current === 'zh' ? 'primary' : 'text'}
        size="small"
        onClick={() => changeLanguage('zh')}
      >
        中文
      </Button>
      <Button
        type={current === 'en' ? 'primary' : 'text'}
        size="small"
        onClick={() => changeLanguage('en')}
      >
        EN
      </Button>
    </Space>
  );
};

export default LanguageSwitcher;
