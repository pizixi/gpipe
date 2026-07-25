import React, { Suspense, lazy, useEffect, useState } from 'react';
import { ConfigProvider, Spin } from 'antd';
import { useAuthStore } from './stores/authStore';
import theme from './theme';
import './i18n';

const LoginPage = lazy(() => import('./pages/Login'));
const DashboardPage = lazy(() => import('./pages/Dashboard'));

const App: React.FC = () => {
  const { isLoggedIn, checking, checkLoginStatus, setLoggedIn } = useAuthStore();
  const [path, setPath] = useState(() => window.location.pathname);

  useEffect(() => {
    checkLoginStatus();
  }, [checkLoginStatus]);

  useEffect(() => {
    const handler = () => setLoggedIn(false);
    window.addEventListener('auth-expired', handler);
    return () => window.removeEventListener('auth-expired', handler);
  }, [setLoggedIn]);

  useEffect(() => {
    const handlePopState = () => setPath(window.location.pathname);
    window.addEventListener('popstate', handlePopState);
    return () => window.removeEventListener('popstate', handlePopState);
  }, []);

  useEffect(() => {
    if (checking) return;
    const target = isLoggedIn ? '/' : '/login';
    const shouldRedirect = isLoggedIn ? path === '/login' : path !== '/login';
    if (shouldRedirect) {
      window.history.replaceState(null, '', target);
      setPath(target);
    }
  }, [checking, isLoggedIn, path]);

  if (checking) {
    return (
      <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
        <Spin size="large" />
      </div>
    );
  }

  return (
    <ConfigProvider theme={theme}>
      <Suspense
        fallback={
          <div style={{ display: 'flex', justifyContent: 'center', alignItems: 'center', height: '100vh' }}>
            <Spin size="large" />
          </div>
        }
      >
        {isLoggedIn ? <DashboardPage /> : <LoginPage />}
      </Suspense>
    </ConfigProvider>
  );
};

export default App;
