import { useState, useEffect } from 'react';
import ChatLayout from './components/ChatLayout';
import SetupWizard from './components/SetupWizard';
import LoginPage from './components/LoginPage';
import MyAccounts from './components/MyAccounts';
import SharedView from './components/SharedView';
import { getStatus } from './api/client';

function getSharedToken(): string | null {
  const path = window.location.pathname;
  const match = path.match(/^\/shared\/([^/]+)/);
  return match ? match[1] : null;
}

function App() {
  const sharedToken = getSharedToken();

  // If this is a shared conversation link, render read-only view immediately
  if (sharedToken) {
    return <SharedView token={sharedToken} />;
  }

  const [state, setState] = useState<'loading' | 'setup' | 'login' | 'chat'>('loading');
  const [page, setPage] = useState<'chat' | 'accounts'>('chat');

  useEffect(() => {
    getStatus().then(s => {
      if (s.needs_setup) setState('setup');
      else if (!s.authenticated) setState('login');
      else setState('chat');
    }).catch(() => setState('login'));
  }, []);

  if (state === 'loading') return <div className="flex items-center justify-center h-screen bg-gray-900 text-white">Loading...</div>;
  if (state === 'setup') return <SetupWizard onComplete={() => setState('chat')} />;
  if (state === 'login') return <LoginPage onLogin={() => setState('chat')} />;
  if (state === 'chat' && page === 'accounts') {
    return <MyAccounts onBack={() => setPage('chat')} />;
  }
  return <ChatLayout onShowAccounts={() => setPage('accounts')} />;
}

export default App;
