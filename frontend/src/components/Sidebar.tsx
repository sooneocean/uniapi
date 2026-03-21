import { useState, useRef, useEffect } from 'react';
import { useTranslation } from 'react-i18next';
import type { Conversation } from '../types';

interface Props {
  conversations: Conversation[];
  activeConversationId: string | null;
  onNewChat: () => void;
  onSelectConversation: (id: string) => void;
  onRegisterFocusSearch?: (fn: () => void) => void;
}

function groupByDate(conversations: Conversation[]): Record<string, Conversation[]> {
  const groups: Record<string, Conversation[]> = {};
  const now = new Date();
  const today = new Date(now.getFullYear(), now.getMonth(), now.getDate());
  const yesterday = new Date(today.getTime() - 86400000);
  const weekAgo = new Date(today.getTime() - 7 * 86400000);

  for (const conv of conversations) {
    const date = new Date(conv.updatedAt);
    const convDay = new Date(date.getFullYear(), date.getMonth(), date.getDate());

    let label: string;
    if (convDay >= today) {
      label = 'Today';
    } else if (convDay >= yesterday) {
      label = 'Yesterday';
    } else if (convDay >= weekAgo) {
      label = 'Previous 7 Days';
    } else {
      label = date.toLocaleDateString('en-US', { month: 'long', year: 'numeric' });
    }

    if (!groups[label]) groups[label] = [];
    groups[label].push(conv);
  }

  return groups;
}

export default function Sidebar({ conversations, activeConversationId, onNewChat, onSelectConversation, onRegisterFocusSearch }: Props) {
  const { t } = useTranslation();
  const [search, setSearch] = useState('');
  const searchRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (onRegisterFocusSearch) {
      onRegisterFocusSearch(() => searchRef.current?.focus());
    }
  }, [onRegisterFocusSearch]);

  const filtered = search.trim()
    ? conversations.filter((c) => c.title.toLowerCase().includes(search.toLowerCase()))
    : conversations;

  const groups = groupByDate(filtered);
  const groupOrder = ['Today', 'Yesterday', 'Previous 7 Days'];
  const otherGroups = Object.keys(groups).filter((g) => !groupOrder.includes(g));
  const orderedGroups = [...groupOrder.filter((g) => groups[g]), ...otherGroups];

  return (
    <div className="w-64 flex flex-col h-full border-r border-gray-700" style={{ background: 'var(--bg-secondary)', color: 'var(--text-primary)' }}>
      {/* Header */}
      <div className="p-4 border-b border-gray-700">
        <h1 className="font-semibold text-lg" style={{ color: 'var(--text-primary)' }}>UniAPI</h1>
      </div>

      {/* Search */}
      <div className="px-3 pt-3">
        <input
          ref={searchRef}
          type="text"
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          placeholder={t('sidebar.search')}
          className="w-full bg-gray-700 text-gray-100 placeholder-gray-500 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500"
        />
      </div>

      {/* New Chat Button */}
      <div className="p-3">
        <button
          onClick={onNewChat}
          className="w-full flex items-center gap-2 px-3 py-2 rounded-lg border border-gray-600 text-gray-300 hover:bg-gray-700 hover:text-white transition-colors text-sm"
        >
          <span className="text-lg leading-none">+</span>
          <span>{t('sidebar.newChat')}</span>
        </button>
      </div>

      {/* Conversation List */}
      <div className="flex-1 overflow-y-auto px-2">
        {conversations.length === 0 ? (
          <p className="text-gray-500 text-sm text-center mt-4 px-2">{t('sidebar.noConversations')}</p>
        ) : filtered.length === 0 ? (
          <p className="text-gray-500 text-sm text-center mt-4 px-2">{t('sidebar.noResults')}</p>
        ) : (
          orderedGroups.map((group) => (
            <div key={group} className="mb-3">
              <p className="text-xs text-gray-500 font-medium px-2 py-1 uppercase tracking-wider">{group}</p>
              {groups[group].map((conv) => (
                <button
                  key={conv.id}
                  onClick={() => onSelectConversation(conv.id)}
                  className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors mb-1 ${
                    activeConversationId === conv.id
                      ? 'bg-gray-600 text-white'
                      : 'text-gray-300 hover:bg-gray-700 hover:text-white'
                  }`}
                  title={conv.title}
                >
                  <div className="truncate">{conv.title}</div>
                  {conv.preview && (
                    <div className="text-xs text-gray-500 truncate mt-0.5">{conv.preview}</div>
                  )}
                </button>
              ))}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
