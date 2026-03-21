import { useState, useRef, useEffect, useMemo } from 'react';
import { useTranslation } from 'react-i18next';
import type { Conversation } from '../types';
import { setFolder, togglePin, deleteConversation } from '../api/client';

interface Props {
  conversations: Conversation[];
  activeConversationId: string | null;
  onNewChat: () => void;
  onSelectConversation: (id: string) => void;
  onConversationsChange?: () => void;
  onRegisterFocusSearch?: (fn: () => void) => void;
}

interface ContextMenu {
  x: number;
  y: number;
  conv: Conversation;
  showFolderInput: boolean;
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

export default function Sidebar({ conversations, activeConversationId, onNewChat, onSelectConversation, onConversationsChange, onRegisterFocusSearch }: Props) {
  const { t } = useTranslation();
  const [searchInput, setSearchInput] = useState('');
  const [debouncedSearch, setDebouncedSearch] = useState('');
  const [contextMenu, setContextMenu] = useState<ContextMenu | null>(null);
  const [collapsedFolders, setCollapsedFolders] = useState<Set<string>>(new Set());
  const [folderInput, setFolderInput] = useState('');
  const searchRef = useRef<HTMLInputElement>(null);
  const contextMenuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (onRegisterFocusSearch) {
      onRegisterFocusSearch(() => searchRef.current?.focus());
    }
  }, [onRegisterFocusSearch]);

  // Debounce search input by 200 ms to avoid filtering on every keystroke
  useEffect(() => {
    const timer = setTimeout(() => setDebouncedSearch(searchInput), 200);
    return () => clearTimeout(timer);
  }, [searchInput]);

  // Close context menu on outside click
  useEffect(() => {
    const handleClick = (e: MouseEvent) => {
      if (contextMenuRef.current && !contextMenuRef.current.contains(e.target as Node)) {
        setContextMenu(null);
      }
    };
    if (contextMenu) {
      document.addEventListener('mousedown', handleClick);
    }
    return () => document.removeEventListener('mousedown', handleClick);
  }, [contextMenu]);

  const filtered = useMemo(() =>
    debouncedSearch.trim()
      ? conversations.filter((c) => c.title.toLowerCase().includes(debouncedSearch.toLowerCase()))
      : conversations,
    [conversations, debouncedSearch]
  );

  // Pinned conversations
  const pinned = filtered.filter((c) => c.pinned);
  // Unpinned, grouped by folder
  const unpinned = filtered.filter((c) => !c.pinned);

  // Get all unique folders
  const allFolders = Array.from(new Set(conversations.map((c) => c.folder || '').filter(Boolean)));

  // Conversations with no folder
  const noFolder = unpinned.filter((c) => !c.folder);
  // Conversations in folders
  const inFolders: Record<string, Conversation[]> = {};
  for (const conv of unpinned) {
    if (conv.folder) {
      if (!inFolders[conv.folder]) inFolders[conv.folder] = [];
      inFolders[conv.folder].push(conv);
    }
  }

  const groups = groupByDate(noFolder);
  const groupOrder = ['Today', 'Yesterday', 'Previous 7 Days'];
  const otherGroups = Object.keys(groups).filter((g) => !groupOrder.includes(g));
  const orderedGroups = [...groupOrder.filter((g) => groups[g]), ...otherGroups];

  const handleContextMenu = (e: React.MouseEvent, conv: Conversation) => {
    e.preventDefault();
    setContextMenu({ x: e.clientX, y: e.clientY, conv, showFolderInput: false });
    setFolderInput('');
  };

  const handleTogglePin = async (conv: Conversation) => {
    setContextMenu(null);
    await togglePin(conv.id).catch(() => {});
    onConversationsChange?.();
  };

  const handleSetFolder = async (conv: Conversation, folder: string) => {
    setContextMenu(null);
    await setFolder(conv.id, folder).catch(() => {});
    onConversationsChange?.();
  };

  const handleDelete = async (conv: Conversation) => {
    setContextMenu(null);
    await deleteConversation(conv.id).catch(() => {});
    onConversationsChange?.();
  };

  const toggleFolder = (folder: string) => {
    setCollapsedFolders((prev) => {
      const next = new Set(prev);
      if (next.has(folder)) next.delete(folder);
      else next.add(folder);
      return next;
    });
  };

  const renderConvButton = (conv: Conversation) => (
    <button
      key={conv.id}
      onClick={() => onSelectConversation(conv.id)}
      onContextMenu={(e) => handleContextMenu(e, conv)}
      className={`w-full text-left px-3 py-2 rounded-lg text-sm transition-colors mb-1 ${
        activeConversationId === conv.id
          ? 'bg-gray-600 text-white'
          : 'text-gray-300 hover:bg-gray-700 hover:text-white'
      }`}
      title={conv.title}
    >
      <div className="truncate flex items-center gap-1">
        {conv.pinned && <span className="text-yellow-400 text-xs">📌</span>}
        {conv.title}
      </div>
      {conv.preview && (
        <div className="text-xs text-gray-500 truncate mt-0.5">{conv.preview}</div>
      )}
    </button>
  );

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
          value={searchInput}
          onChange={(e) => setSearchInput(e.target.value)}
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
          <>
            {/* Pinned section */}
            {pinned.length > 0 && (
              <div className="mb-3">
                <p className="text-xs text-yellow-500 font-medium px-2 py-1 uppercase tracking-wider">Pinned</p>
                {pinned.map(renderConvButton)}
              </div>
            )}

            {/* Folders */}
            {Object.keys(inFolders).sort().map((folder) => (
              <div key={folder} className="mb-3">
                <button
                  className="w-full flex items-center gap-1 text-xs text-blue-400 font-medium px-2 py-1 uppercase tracking-wider hover:text-blue-300"
                  onClick={() => toggleFolder(folder)}
                >
                  <span>{collapsedFolders.has(folder) ? '▶' : '▼'}</span>
                  <span>📁 {folder}</span>
                </button>
                {!collapsedFolders.has(folder) && inFolders[folder].map(renderConvButton)}
              </div>
            ))}

            {/* No-folder conversations grouped by date */}
            {orderedGroups.map((group) => (
              <div key={group} className="mb-3">
                <p className="text-xs text-gray-500 font-medium px-2 py-1 uppercase tracking-wider">{group}</p>
                {groups[group].map(renderConvButton)}
              </div>
            ))}
          </>
        )}
      </div>

      {/* Context Menu */}
      {contextMenu && (
        <div
          ref={contextMenuRef}
          className="fixed z-50 bg-gray-800 border border-gray-600 rounded-lg shadow-xl py-1 min-w-[160px]"
          style={{ top: contextMenu.y, left: Math.min(contextMenu.x, window.innerWidth - 200) }}
        >
          <button
            className="w-full text-left px-4 py-2 text-sm text-gray-300 hover:bg-gray-700"
            onClick={() => handleTogglePin(contextMenu.conv)}
          >
            {contextMenu.conv.pinned ? 'Unpin' : '📌 Pin'}
          </button>

          {!contextMenu.showFolderInput ? (
            <button
              className="w-full text-left px-4 py-2 text-sm text-gray-300 hover:bg-gray-700"
              onClick={() => setContextMenu({ ...contextMenu, showFolderInput: true })}
            >
              📁 Move to folder
            </button>
          ) : (
            <div className="px-3 py-2">
              {allFolders.length > 0 && (
                <div className="mb-1">
                  {allFolders.map((f) => (
                    <button
                      key={f}
                      className="w-full text-left px-2 py-1 text-xs text-gray-300 hover:bg-gray-700 rounded"
                      onClick={() => handleSetFolder(contextMenu.conv, f)}
                    >
                      {f}
                    </button>
                  ))}
                </div>
              )}
              <input
                autoFocus
                className="w-full bg-gray-700 text-gray-100 border border-gray-600 rounded px-2 py-1 text-xs focus:outline-none"
                placeholder="New folder name..."
                value={folderInput}
                onChange={(e) => setFolderInput(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === 'Enter' && folderInput.trim()) {
                    handleSetFolder(contextMenu.conv, folderInput.trim());
                  } else if (e.key === 'Escape') {
                    setContextMenu(null);
                  }
                }}
              />
            </div>
          )}

          {contextMenu.conv.folder && (
            <button
              className="w-full text-left px-4 py-2 text-sm text-gray-300 hover:bg-gray-700"
              onClick={() => handleSetFolder(contextMenu.conv, '')}
            >
              Remove from folder
            </button>
          )}

          <hr className="border-gray-700 my-1" />
          <button
            className="w-full text-left px-4 py-2 text-sm text-red-400 hover:bg-gray-700"
            onClick={() => handleDelete(contextMenu.conv)}
          >
            Delete
          </button>
        </div>
      )}
    </div>
  );
}
