import { useState } from 'react';
import { bindSessionToken } from '../api/client';

interface Props {
  provider: string;
  displayName: string;
  shared: boolean;
  onClose: () => void;
  onSuccess: () => void;
}

export default function SessionTokenDialog({ provider, displayName, shared, onClose, onSuccess }: Props) {
  const [token, setToken] = useState('');
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(false);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError('');
    setLoading(true);
    try {
      await bindSessionToken(provider, token, shared);
      onSuccess();
    } catch (err: any) {
      setError(err?.response?.data?.error ?? 'Failed to bind session token');
    } finally {
      setLoading(false);
    }
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black bg-opacity-60">
      <div className="bg-gray-800 rounded-xl p-6 w-full max-w-md shadow-xl">
        <h2 className="text-white text-lg font-semibold mb-4">Bind {displayName} Session Token</h2>

        <div className="mb-4 text-sm text-gray-400 space-y-1">
          <p className="font-medium text-gray-300">Instructions:</p>
          <ol className="list-decimal list-inside space-y-1">
            <li>Log in to {displayName}</li>
            <li>Open Developer Tools → Application</li>
            <li>Find session token in cookies</li>
          </ol>
        </div>

        <form onSubmit={handleSubmit} className="space-y-4">
          <div>
            <label className="block text-sm text-gray-300 mb-1">Session Token</label>
            <textarea
              value={token}
              onChange={(e) => setToken(e.target.value)}
              rows={4}
              required
              placeholder="Paste your session token here..."
              className="w-full bg-gray-700 text-white rounded px-3 py-2 text-sm border border-gray-600 focus:outline-none focus:border-blue-500 placeholder-gray-500 resize-none"
            />
          </div>

          {error && <p className="text-red-400 text-sm">{error}</p>}

          <div className="flex gap-3 justify-end">
            <button
              type="button"
              onClick={onClose}
              className="px-4 py-2 text-sm text-gray-300 bg-gray-700 rounded-lg hover:bg-gray-600 transition-colors"
            >
              Cancel
            </button>
            <button
              type="submit"
              disabled={loading || !token.trim()}
              className="px-4 py-2 text-sm text-white bg-blue-600 rounded-lg hover:bg-blue-500 disabled:opacity-50 transition-colors"
            >
              {loading ? 'Binding...' : 'Bind'}
            </button>
          </div>
        </form>
      </div>
    </div>
  );
}
