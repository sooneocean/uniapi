import { useState, useEffect } from 'react';
import ChatLayout from './components/ChatLayout';
import SetupWizard from './components/SetupWizard';
import LoginPage from './components/LoginPage';
import { getStatus } from './api/client';

function App() {
  const [state, setState] = useState<'loading' | 'setup' | 'login' | 'chat'>('loading');

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
  return <ChatLayout />;
}

export default App;
