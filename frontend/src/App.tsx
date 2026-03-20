import { useState, useEffect } from 'react';
import ChatLayout from './components/ChatLayout';
import SetupWizard from './components/SetupWizard';
import LoginPage from './components/LoginPage';
import MyAccounts from './components/MyAccounts';
import { getStatus } from './api/client';

function App() {
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
