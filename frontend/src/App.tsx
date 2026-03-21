import { lazy, Suspense, useState, useEffect } from 'react';
import { getStatus } from './api/client';
import './i18n';
import { ToastProvider } from './components/Toast';

// Eager load (small, always needed)
import LoginPage from './components/LoginPage';
import SetupWizard from './components/SetupWizard';

// Lazy load (heavy, conditionally needed)
const ChatLayout = lazy(() => import('./components/ChatLayout'));
const MyAccounts = lazy(() => import('./components/MyAccounts'));
const APIPlayground = lazy(() => import('./components/APIPlayground'));
const SharedView = lazy(() => import('./components/SharedView'));

function getSharedToken(): string | null {
  const path = window.location.pathname;
  const match = path.match(/^\/shared\/([^/]+)/);
  return match ? match[1] : null;
}

const Loading = () => (
  <div className="flex items-center justify-center h-screen bg-gray-900 text-white">
    Loading...
  </div>
);

function App() {
  const sharedToken = getSharedToken();

  // If this is a shared conversation link, render read-only view immediately
  if (sharedToken) {
    return (
      <ToastProvider>
        <Suspense fallback={<Loading />}>
          <SharedView token={sharedToken} />
        </Suspense>
      </ToastProvider>
    );
  }

  const [state, setState] = useState<'loading' | 'setup' | 'login' | 'chat'>('loading');
  const [page, setPage] = useState<'chat' | 'accounts' | 'playground'>('chat');

  useEffect(() => {
    getStatus().then(s => {
      if (s.needs_setup) setState('setup');
      else if (!s.authenticated) setState('login');
      else setState('chat');
    }).catch(() => setState('login'));
  }, []);

  if (state === 'loading') return <Loading />;
  if (state === 'setup') return <SetupWizard onComplete={() => setState('chat')} />;
  if (state === 'login') return <LoginPage onLogin={() => setState('chat')} />;

  return (
    <ToastProvider>
      <Suspense fallback={<Loading />}>
        {state === 'chat' && page === 'accounts' && (
          <MyAccounts onBack={() => setPage('chat')} />
        )}
        {state === 'chat' && page === 'playground' && (
          <APIPlayground onBack={() => setPage('chat')} />
        )}
        {state === 'chat' && page === 'chat' && (
          <ChatLayout onShowAccounts={() => setPage('accounts')} onShowPlayground={() => setPage('playground')} />
        )}
      </Suspense>
    </ToastProvider>
  );
}

export default App;
