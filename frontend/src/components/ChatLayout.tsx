import { useState, useEffect } from 'react';
import type { Conversation } from '../types';
import Sidebar from './Sidebar';
import ChatArea from './ChatArea';
import Settings from './Settings';
import ThemeToggle from './ThemeToggle';
import { getMe, logout, getConversations, createConversation } from '../api/client';
import { useTheme } from '../hooks/useTheme';

interface Props {
  onShowAccounts?: () => void;
}

export default function ChatLayout({ onShowAccounts }: Props) {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConversationId, setActiveConversationId] = useState<string | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [userRole, setUserRole] = useState<string>('member');
  const [sidebarOpen, setSidebarOpen] = useState(false);
  const { theme, toggle: toggleTheme } = useTheme();

  useEffect(() => {
    getMe().then((me) => setUserRole(me.role)).catch(() => {});
  }, []);

  useEffect(() => {
    getConversations()
      .then((convs) => setConversations(convs))
      .catch(() => {});
  }, []);

  const handleNewChat = async () => {
    try {
      const conv = await createConversation('New Chat');
      setConversations((prev) => [conv, ...prev]);
      setActiveConversationId(conv.id);
    } catch {
      setActiveConversationId(null);
    }
    setSidebarOpen(false);
  };

  const handleSelectConversation = (id: string) => {
    setActiveConversationId(id);
    setSidebarOpen(false);
  };

  const handleConversationTitleUpdate = (id: string, title: string) => {
    setConversations((prev) =>
      prev.map((c) => (c.id === id ? { ...c, title } : c))
    );
  };

  const handleLogout = async () => {
    try {
      await logout();
    } finally {
      window.location.reload();
    }
  };

  return (
    <div className="flex h-screen w-screen overflow-hidden" style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)' }}>
      {/* Mobile overlay */}
      {sidebarOpen && (
        <div
          className="fixed inset-0 bg-black/50 z-40 md:hidden"
          onClick={() => setSidebarOpen(false)}
        />
      )}

      {/* Sidebar */}
      <div
        className={`fixed md:static z-50 h-full transition-transform duration-200 ${
          sidebarOpen ? 'translate-x-0' : '-translate-x-full md:translate-x-0'
        } w-64 flex-shrink-0`}
      >
        <Sidebar
          conversations={conversations}
          activeConversationId={activeConversationId}
          onNewChat={handleNewChat}
          onSelectConversation={handleSelectConversation}
        />
      </div>

      {/* Main content */}
      <div className="flex-1 overflow-hidden flex flex-col min-w-0">
        {/* Header bar */}
        <div className="flex items-center justify-between gap-2 px-4 py-2 border-b border-gray-700" style={{ background: 'var(--bg-secondary)' }}>
          {/* Hamburger button (mobile only) */}
          <button
            className="md:hidden text-gray-400 hover:text-white transition-colors text-xl px-2 py-1 rounded hover:bg-gray-700"
            onClick={() => setSidebarOpen(!sidebarOpen)}
            aria-label="Toggle sidebar"
          >
            &#9776;
          </button>
          <div className="hidden md:block" />

          {/* Right side icons */}
          <div className="flex items-center gap-2">
            <ThemeToggle theme={theme} onToggle={toggleTheme} />
            <button
              onClick={() => onShowAccounts?.()}
              title="My Accounts"
              className="text-gray-400 hover:text-white transition-colors text-lg px-2 py-1 rounded hover:bg-gray-700"
              aria-label="My Accounts"
            >
              &#128100;
            </button>
            <button
              onClick={() => setShowSettings(true)}
              title="Settings"
              className="text-gray-400 hover:text-white transition-colors text-lg px-2 py-1 rounded hover:bg-gray-700"
              aria-label="Open settings"
            >
              &#9881;
            </button>
            <button
              onClick={handleLogout}
              title="Logout"
              className="text-gray-400 hover:text-white transition-colors text-sm px-3 py-1 rounded hover:bg-gray-700 border border-gray-600"
            >
              Logout
            </button>
          </div>
        </div>

        <div className="flex-1 overflow-hidden">
          <ChatArea
            conversationId={activeConversationId}
            onConversationTitleUpdate={handleConversationTitleUpdate}
          />
        </div>
      </div>

      {showSettings && (
        <Settings onClose={() => setShowSettings(false)} userRole={userRole} />
      )}
    </div>
  );
}
