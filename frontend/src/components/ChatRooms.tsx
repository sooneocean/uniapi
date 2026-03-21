import { useState, useEffect, useRef } from 'react';
import {
  createRoom, getRooms, joinRoom, getRoomMessages, sendRoomMessage, deleteRoom,
} from '../api/client';

interface Room {
  id: string;
  name: string;
  created_by: string;
  created_at: string;
}

interface RoomMessage {
  id: string;
  room_id: string;
  user_id?: string;
  username: string;
  role: string;
  content: string;
  model?: string;
  created_at: string;
}

interface ChatRoomsProps {
  userID?: string;
  username?: string;
}

export default function ChatRooms(_props: ChatRoomsProps = {}) {
  const [rooms, setRooms] = useState<Room[]>([]);
  const [activeRoom, setActiveRoom] = useState<Room | null>(null);
  const [messages, setMessages] = useState<RoomMessage[]>([]);
  const [input, setInput] = useState('');
  const [model, setModel] = useState('');
  const [newRoomName, setNewRoomName] = useState('');
  const [joinID, setJoinID] = useState('');
  const [loading, setLoading] = useState(false);
  const [showCreate, setShowCreate] = useState(false);
  const [members, setMembers] = useState<{ id: string; username: string }[]>([]);
  const [sseConnected, setSseConnected] = useState(false);
  const messagesEndRef = useRef<HTMLDivElement>(null);
  const sseRef = useRef<EventSource | null>(null);
  const activeRoomIdRef = useRef<string | null>(null);

  const loadRooms = async () => {
    try {
      const data = await getRooms();
      setRooms(data || []);
    } catch {}
  };

  useEffect(() => { loadRooms(); }, []);

  useEffect(() => {
    messagesEndRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages]);

  // SSE: connect/disconnect when active room changes
  useEffect(() => {
    // Disconnect previous
    if (sseRef.current) {
      sseRef.current.close();
      sseRef.current = null;
      setSseConnected(false);
    }
    if (!activeRoom) return;

    activeRoomIdRef.current = activeRoom.id;
    const es = new EventSource(`/api/rooms/${activeRoom.id}/stream`, { withCredentials: true });
    sseRef.current = es;

    es.onopen = () => setSseConnected(true);
    es.onerror = () => {
      setSseConnected(false);
      // Reconnect after 3s
      setTimeout(() => {
        if (activeRoomIdRef.current === activeRoom.id) {
          es.close();
          // re-trigger by toggling state — use a fresh EventSource
          sseRef.current = null;
        }
      }, 3000);
    };
    es.onmessage = (event) => {
      try {
        const msg: RoomMessage = JSON.parse(event.data);
        setMessages(prev => {
          // Avoid duplicates
          if (prev.some(m => m.id === msg.id)) return prev;
          return [...prev, msg];
        });
      } catch {}
    };

    return () => {
      es.close();
      sseRef.current = null;
      setSseConnected(false);
    };
  }, [activeRoom?.id]);

  const selectRoom = async (room: Room) => {
    setActiveRoom(room);
    try {
      const msgs = await getRoomMessages(room.id);
      setMessages(msgs || []);
    } catch {}
    try {
      const resp = await fetch(`/api/rooms/${room.id}/members`, { credentials: 'include' });
      if (resp.ok) setMembers(await resp.json());
    } catch {}
  };

  const handleCreateRoom = async () => {
    if (!newRoomName.trim()) return;
    try {
      await createRoom(newRoomName.trim());
      setNewRoomName('');
      setShowCreate(false);
      await loadRooms();
    } catch {}
  };

  const handleJoinRoom = async () => {
    if (!joinID.trim()) return;
    try {
      await joinRoom(joinID.trim());
      setJoinID('');
      await loadRooms();
    } catch {}
  };

  const handleSend = async () => {
    if (!input.trim() || !activeRoom) return;
    const msg = input.trim();
    setInput('');
    setLoading(true);
    try {
      const resp = await sendRoomMessage(activeRoom.id, msg, model || undefined);
      const newMsgs: RoomMessage[] = [];
      if (resp.message) newMsgs.push(resp.message);
      if (resp.ai_response) newMsgs.push(resp.ai_response);
      setMessages(prev => [...prev, ...newMsgs]);
    } catch {
    } finally {
      setLoading(false);
    }
  };

  const handleDeleteRoom = async (room: Room) => {
    if (!confirm(`Delete room "${room.name}"?`)) return;
    try {
      await deleteRoom(room.id);
      if (activeRoom?.id === room.id) {
        setActiveRoom(null);
        setMessages([]);
      }
      await loadRooms();
    } catch {}
  };

  const handleKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      handleSend();
    }
  };

  const getInitial = (name: string) => name.charAt(0).toUpperCase();

  return (
    <div className="flex h-full" style={{ minHeight: '500px' }}>
      {/* Sidebar */}
      <div className="w-56 flex-shrink-0 flex flex-col border-r" style={{ borderColor: 'var(--border-color)' }}>
        <div className="px-3 py-3 border-b flex items-center justify-between" style={{ borderColor: 'var(--border-color)' }}>
          <span className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Rooms</span>
          <button
            onClick={() => setShowCreate(!showCreate)}
            className="text-xs px-2 py-1 rounded"
            style={{ background: 'var(--accent-color)', color: 'white' }}
          >+ New</button>
        </div>

        {showCreate && (
          <div className="px-3 py-2 space-y-1 border-b" style={{ borderColor: 'var(--border-color)' }}>
            <input
              value={newRoomName}
              onChange={e => setNewRoomName(e.target.value)}
              onKeyDown={e => e.key === 'Enter' && handleCreateRoom()}
              placeholder="Room name"
              className="w-full px-2 py-1 text-xs rounded"
              style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
            />
            <button onClick={handleCreateRoom} className="w-full text-xs py-1 rounded" style={{ background: 'var(--accent-color)', color: 'white' }}>Create</button>
          </div>
        )}

        <div className="flex-1 overflow-y-auto">
          {rooms.map(room => (
            <div
              key={room.id}
              className="group flex items-center justify-between px-3 py-2 cursor-pointer"
              style={{
                background: activeRoom?.id === room.id ? 'var(--bg-tertiary)' : 'transparent',
                color: 'var(--text-primary)',
              }}
              onClick={() => selectRoom(room)}
            >
              <span className="text-sm truncate">{room.name}</span>
              <button
                className="opacity-0 group-hover:opacity-100 text-red-400 text-xs ml-1"
                onClick={e => { e.stopPropagation(); handleDeleteRoom(room); }}
                title="Delete room"
              >✕</button>
            </div>
          ))}
        </div>

        <div className="px-3 py-2 border-t space-y-1" style={{ borderColor: 'var(--border-color)' }}>
          <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>Join by ID:</p>
          <div className="flex gap-1">
            <input
              value={joinID}
              onChange={e => setJoinID(e.target.value)}
              placeholder="Room ID"
              className="flex-1 px-2 py-1 text-xs rounded"
              style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
            />
            <button onClick={handleJoinRoom} className="text-xs px-2 rounded" style={{ background: 'var(--accent-color)', color: 'white' }}>Join</button>
          </div>
        </div>
      </div>

      {/* Main chat area */}
      {activeRoom ? (
        <div className="flex-1 flex flex-col">
          {/* Room header */}
          <div className="px-4 py-3 border-b flex items-center justify-between" style={{ borderColor: 'var(--border-color)' }}>
            <div>
              <div className="flex items-center gap-2">
                <p className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>{activeRoom.name}</p>
                {sseConnected ? (
                  <span className="text-xs flex items-center gap-1" style={{ color: '#22c55e' }}>
                    <span className="inline-block w-1.5 h-1.5 rounded-full bg-green-400 animate-pulse" />
                    Live
                  </span>
                ) : (
                  <span className="text-xs" style={{ color: 'var(--text-secondary)' }}>Connecting...</span>
                )}
              </div>
              <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                {members.length} member{members.length !== 1 ? 's' : ''}: {members.map(m => m.username).join(', ')}
              </p>
            </div>
            <div className="flex items-center gap-2">
              <input
                value={model}
                onChange={e => setModel(e.target.value)}
                placeholder="Model for @ai"
                className="px-2 py-1 text-xs rounded"
                style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)', width: '140px' }}
              />
            </div>
          </div>

          {/* Messages */}
          <div className="flex-1 overflow-y-auto px-4 py-3 space-y-3">
            {messages.map(msg => (
              <div key={msg.id} className="flex items-start gap-2">
                <div
                  className="w-7 h-7 rounded-full flex items-center justify-center text-xs font-bold flex-shrink-0"
                  style={{
                    background: msg.role === 'assistant' ? 'var(--accent-color)' : 'var(--bg-tertiary)',
                    color: msg.role === 'assistant' ? 'white' : 'var(--text-primary)',
                  }}
                >
                  {getInitial(msg.username || 'A')}
                </div>
                <div className="flex-1">
                  <div className="flex items-baseline gap-2">
                    <span className="text-xs font-semibold" style={{ color: 'var(--text-secondary)' }}>
                      {msg.username}
                    </span>
                    {msg.model && (
                      <span className="text-xs px-1 rounded" style={{ background: 'var(--bg-tertiary)', color: 'var(--text-secondary)' }}>
                        {msg.model}
                      </span>
                    )}
                    <span className="text-xs" style={{ color: 'var(--text-tertiary, var(--text-secondary))' }}>
                      {new Date(msg.created_at).toLocaleTimeString()}
                    </span>
                  </div>
                  <p className="text-sm mt-0.5 whitespace-pre-wrap" style={{ color: 'var(--text-primary)' }}>{msg.content}</p>
                </div>
              </div>
            ))}
            <div ref={messagesEndRef} />
          </div>

          {/* Input */}
          <div className="px-4 py-3 border-t" style={{ borderColor: 'var(--border-color)' }}>
            <p className="text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>
              Tip: start with <code>@ai</code> to trigger an AI response
            </p>
            <div className="flex gap-2">
              <textarea
                value={input}
                onChange={e => setInput(e.target.value)}
                onKeyDown={handleKeyDown}
                placeholder="Type a message... (@ai to get AI response)"
                rows={2}
                className="flex-1 px-3 py-2 text-sm rounded resize-none"
                style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
              />
              <button
                onClick={handleSend}
                disabled={loading || !input.trim()}
                className="px-4 rounded text-sm font-medium disabled:opacity-50"
                style={{ background: 'var(--accent-color)', color: 'white' }}
              >
                {loading ? '...' : 'Send'}
              </button>
            </div>
          </div>
        </div>
      ) : (
        <div className="flex-1 flex items-center justify-center" style={{ color: 'var(--text-secondary)' }}>
          <p className="text-sm">Select or create a room to start chatting</p>
        </div>
      )}
    </div>
  );
}
