import type { Conversation } from '../types';

interface Props {
  conversations: Conversation[];
  activeConversationId: string | null;
  onNewChat: () => void;
  onSelectConversation: (id: string) => void;
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

export default function Sidebar({ conversations, activeConversationId, onNewChat, onSelectConversation }: Props) {
  const groups = groupByDate(conversations);
  const groupOrder = ['Today', 'Yesterday', 'Previous 7 Days'];
  const otherGroups = Object.keys(groups).filter((g) => !groupOrder.includes(g));
  const orderedGroups = [...groupOrder.filter((g) => groups[g]), ...otherGroups];

  return (
    <div className="w-64 bg-gray-800 flex flex-col h-full border-r border-gray-700">
      {/* Header */}
      <div className="p-4 border-b border-gray-700">
        <h1 className="text-white font-semibold text-lg">UniAPI</h1>
      </div>

      {/* New Chat Button */}
      <div className="p-3">
        <button
          onClick={onNewChat}
          className="w-full flex items-center gap-2 px-3 py-2 rounded-lg border border-gray-600 text-gray-300 hover:bg-gray-700 hover:text-white transition-colors text-sm"
        >
          <span className="text-lg leading-none">+</span>
          <span>New Chat</span>
        </button>
      </div>

      {/* Conversation List */}
      <div className="flex-1 overflow-y-auto px-2">
        {conversations.length === 0 ? (
          <p className="text-gray-500 text-sm text-center mt-4 px-2">No conversations yet</p>
        ) : (
          orderedGroups.map((group) => (
            <div key={group} className="mb-3">
              <p className="text-xs text-gray-500 font-medium px-2 py-1 uppercase tracking-wider">{group}</p>
              {groups[group].map((conv) => (
                <button
                  key={conv.id}
                  onClick={() => onSelectConversation(conv.id)}
                  className={`w-full text-left px-3 py-2 rounded-lg text-sm truncate transition-colors mb-1 ${
                    activeConversationId === conv.id
                      ? 'bg-gray-600 text-white'
                      : 'text-gray-300 hover:bg-gray-700 hover:text-white'
                  }`}
                  title={conv.title}
                >
                  {conv.title}
                </button>
              ))}
            </div>
          ))
        )}
      </div>
    </div>
  );
}
