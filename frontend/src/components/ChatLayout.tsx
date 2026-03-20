import { useState, useEffect } from 'react';
import type { Conversation } from '../types';
import Sidebar from './Sidebar';
import ChatArea from './ChatArea';
import Settings from './Settings';
import { getMe, logout, getConversations, createConversation } from '../api/client';

export default function ChatLayout() {
  const [conversations, setConversations] = useState<Conversation[]>([]);
  const [activeConversationId, setActiveConversationId] = useState<string | null>(null);
  const [showSettings, setShowSettings] = useState(false);
  const [userRole, setUserRole] = useState<string>('member');

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
  };

  const handleSelectConversation = (id: string) => {
    setActiveConversationId(id);
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
    <div className="flex h-screen w-screen bg-gray-900 overflow-hidden">
      <Sidebar
        conversations={conversations}
        activeConversationId={activeConversationId}
        onNewChat={handleNewChat}
        onSelectConversation={handleSelectConversation}
      />
      <div className="flex-1 overflow-hidden flex flex-col">
        {/* Header bar with gear and logout */}
        <div className="flex items-center justify-end gap-2 px-4 py-2 bg-gray-800 border-b border-gray-700">
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
