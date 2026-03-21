import { useState, useEffect, useRef } from 'react';
import { uploadKnowledge, getKnowledge, deleteKnowledge } from '../api/client';

interface KnowledgeDoc {
  id: string;
  user_id: string;
  title: string;
  chunk_count: number;
  shared: boolean;
  created_at: string;
}

export default function KnowledgeBase() {
  const [docs, setDocs] = useState<KnowledgeDoc[]>([]);
  const [title, setTitle] = useState('');
  const [content, setContent] = useState('');
  const [shared, setShared] = useState(false);
  const [uploading, setUploading] = useState(false);
  const [error, setError] = useState('');
  const fileRef = useRef<HTMLInputElement>(null);

  const load = async () => {
    try {
      const data = await getKnowledge();
      setDocs(data || []);
    } catch {}
  };

  useEffect(() => { load(); }, []);

  const handleFileChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    if (!title) setTitle(file.name.replace(/\.[^.]+$/, ''));
    const reader = new FileReader();
    reader.onload = (ev) => setContent(ev.target?.result as string ?? '');
    reader.readAsText(file);
  };

  const handleUpload = async () => {
    if (!title.trim() || !content.trim()) {
      setError('Title and content are required.');
      return;
    }
    setUploading(true);
    setError('');
    try {
      await uploadKnowledge(title.trim(), content.trim(), shared);
      setTitle('');
      setContent('');
      setShared(false);
      if (fileRef.current) fileRef.current.value = '';
      await load();
    } catch (e: any) {
      setError(e?.response?.data?.error || 'Upload failed');
    } finally {
      setUploading(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this document?')) return;
    try {
      await deleteKnowledge(id);
      await load();
    } catch {}
  };

  return (
    <div className="space-y-6">
      <div className="rounded-lg p-4 space-y-3" style={{ background: 'var(--bg-tertiary)' }}>
        <h3 className="text-sm font-semibold" style={{ color: 'var(--text-primary)' }}>Upload Document</h3>

        <input
          type="text"
          placeholder="Document title"
          value={title}
          onChange={e => setTitle(e.target.value)}
          className="w-full px-3 py-2 rounded text-sm"
          style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
        />

        <div>
          <label className="block text-xs mb-1" style={{ color: 'var(--text-secondary)' }}>Upload text file (optional)</label>
          <input
            ref={fileRef}
            type="file"
            accept=".txt,.md,.csv,.json"
            onChange={handleFileChange}
            className="w-full text-xs"
            style={{ color: 'var(--text-secondary)' }}
          />
        </div>

        <textarea
          placeholder="Or paste document content here..."
          value={content}
          onChange={e => setContent(e.target.value)}
          rows={5}
          className="w-full px-3 py-2 rounded text-sm resize-y"
          style={{ background: 'var(--bg-primary)', color: 'var(--text-primary)', border: '1px solid var(--border-color)' }}
        />

        <label className="flex items-center gap-2 text-sm cursor-pointer" style={{ color: 'var(--text-secondary)' }}>
          <input type="checkbox" checked={shared} onChange={e => setShared(e.target.checked)} />
          Share with all users
        </label>

        {error && <p className="text-red-400 text-sm">{error}</p>}

        <button
          onClick={handleUpload}
          disabled={uploading}
          className="px-4 py-2 rounded text-sm font-medium disabled:opacity-50"
          style={{ background: 'var(--accent-color)', color: 'white' }}
        >
          {uploading ? 'Uploading...' : 'Upload'}
        </button>
      </div>

      <div>
        <h3 className="text-sm font-semibold mb-3" style={{ color: 'var(--text-primary)' }}>
          Documents ({docs.length})
        </h3>
        {docs.length === 0 ? (
          <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>No documents uploaded yet.</p>
        ) : (
          <div className="space-y-2">
            {docs.map(doc => (
              <div key={doc.id} className="flex items-center justify-between rounded px-3 py-2"
                style={{ background: 'var(--bg-tertiary)', border: '1px solid var(--border-color)' }}>
                <div>
                  <p className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>{doc.title}</p>
                  <p className="text-xs" style={{ color: 'var(--text-secondary)' }}>
                    {doc.chunk_count} chunk{doc.chunk_count !== 1 ? 's' : ''}
                    {doc.shared && ' · shared'}
                    {' · '}{new Date(doc.created_at).toLocaleDateString()}
                  </p>
                </div>
                <button
                  onClick={() => handleDelete(doc.id)}
                  className="text-red-400 hover:text-red-300 text-xs px-2 py-1 rounded"
                  title="Delete document"
                >
                  Delete
                </button>
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
