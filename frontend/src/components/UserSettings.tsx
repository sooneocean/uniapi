import { useState, useEffect } from 'react';
import { getUsers, addUser, deleteUser, getMe, updateUserQuotas } from '../api/client';

interface User {
  id: string;
  username: string;
  role: string;
  created_at?: string;
  daily_token_limit?: number;
  daily_cost_limit?: number;
  monthly_cost_limit?: number;
}

export default function UserSettings() {
  const [users, setUsers] = useState<User[]>([]);
  const [me, setMe] = useState<{ id: string; username: string; role: string } | null>(null);
  const [loading, setLoading] = useState(true);
  const [showForm, setShowForm] = useState(false);
  const [error, setError] = useState('');

  const [formUsername, setFormUsername] = useState('');
  const [formPassword, setFormPassword] = useState('');
  const [formRole, setFormRole] = useState('member');
  const [submitting, setSubmitting] = useState(false);

  // Quota editing state
  const [editingQuotaId, setEditingQuotaId] = useState<string | null>(null);
  const [quotaDailyTokens, setQuotaDailyTokens] = useState('0');
  const [quotaDailyCost, setQuotaDailyCost] = useState('0');
  const [quotaMonthlyCost, setQuotaMonthlyCost] = useState('0');
  const [quotaSubmitting, setQuotaSubmitting] = useState(false);

  const load = async () => {
    try {
      setLoading(true);
      const [usersData, meData] = await Promise.all([getUsers(), getMe()]);
      setUsers(Array.isArray(usersData) ? usersData : usersData.users ?? []);
      setMe(meData);
    } catch {
      setError('Failed to load users');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleAdd = async (e: React.FormEvent) => {
    e.preventDefault();
    setSubmitting(true);
    setError('');
    try {
      await addUser({ username: formUsername, password: formPassword, role: formRole });
      setShowForm(false);
      setFormUsername('');
      setFormPassword('');
      setFormRole('member');
      await load();
    } catch {
      setError('Failed to add user');
    } finally {
      setSubmitting(false);
    }
  };

  const handleDelete = async (id: string) => {
    if (!confirm('Delete this user?')) return;
    try {
      await deleteUser(id);
      await load();
    } catch {
      setError('Failed to delete user');
    }
  };

  const startEditQuota = (u: User) => {
    setEditingQuotaId(u.id);
    setQuotaDailyTokens(String(u.daily_token_limit ?? 0));
    setQuotaDailyCost(String(u.daily_cost_limit ?? 0));
    setQuotaMonthlyCost(String(u.monthly_cost_limit ?? 0));
  };

  const handleSaveQuota = async (id: string) => {
    setQuotaSubmitting(true);
    try {
      await updateUserQuotas(id, {
        daily_token_limit: parseInt(quotaDailyTokens) || 0,
        daily_cost_limit: parseFloat(quotaDailyCost) || 0,
        monthly_cost_limit: parseFloat(quotaMonthlyCost) || 0,
      });
      setEditingQuotaId(null);
      await load();
    } catch {
      setError('Failed to update quotas');
    } finally {
      setQuotaSubmitting(false);
    }
  };

  const formatDate = (dateStr?: string) => {
    if (!dateStr) return '—';
    return new Date(dateStr).toLocaleDateString();
  };

  const formatQuota = (val?: number, prefix = '') => {
    if (!val || val === 0) return '∞';
    return prefix + val.toLocaleString();
  };

  return (
    <div className="text-white">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Users</h2>
        <button
          onClick={() => setShowForm(!showForm)}
          className="px-3 py-1.5 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 transition-colors"
        >
          {showForm ? 'Cancel' : '+ Add User'}
        </button>
      </div>

      {error && <p className="text-red-400 text-sm mb-3">{error}</p>}

      {showForm && (
        <form onSubmit={handleAdd} className="bg-gray-700 rounded-lg p-4 mb-4 space-y-3">
          <div>
            <label className="block text-sm text-gray-300 mb-1">Username</label>
            <input
              type="text"
              value={formUsername}
              onChange={(e) => setFormUsername(e.target.value)}
              required
              placeholder="johndoe"
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
            />
          </div>
          <div>
            <label className="block text-sm text-gray-300 mb-1">Password</label>
            <input
              type="password"
              value={formPassword}
              onChange={(e) => setFormPassword(e.target.value)}
              required
              placeholder="••••••••"
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500 placeholder-gray-400"
            />
          </div>
          <div>
            <label className="block text-sm text-gray-300 mb-1">Role</label>
            <select
              value={formRole}
              onChange={(e) => setFormRole(e.target.value)}
              className="w-full bg-gray-600 text-white rounded px-3 py-2 text-sm border border-gray-500 focus:outline-none focus:border-blue-500"
            >
              <option value="member">Member</option>
              <option value="admin">Admin</option>
            </select>
          </div>
          <button
            type="submit"
            disabled={submitting}
            className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 disabled:opacity-50 transition-colors"
          >
            {submitting ? 'Adding...' : 'Add User'}
          </button>
        </form>
      )}

      {loading ? (
        <p className="text-gray-400 text-sm">Loading...</p>
      ) : users.length === 0 ? (
        <p className="text-gray-400 text-sm">No users found.</p>
      ) : (
        <div className="space-y-2">
          {users.map((u) => (
            <div key={u.id} className="bg-gray-700 rounded-lg p-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="text-white font-medium">{u.username}</span>
                  <span className={`text-xs px-2 py-0.5 rounded-full font-medium ${u.role === 'admin' ? 'bg-purple-900 text-purple-300' : 'bg-gray-600 text-gray-300'}`}>
                    {u.role}
                  </span>
                  <span className="text-gray-400 text-xs">Joined {formatDate(u.created_at)}</span>
                </div>
                <div className="flex items-center gap-2">
                  <button
                    onClick={() => editingQuotaId === u.id ? setEditingQuotaId(null) : startEditQuota(u)}
                    className="px-3 py-1.5 bg-gray-600 text-gray-200 rounded text-sm hover:bg-gray-500 transition-colors"
                  >
                    Quotas
                  </button>
                  <button
                    onClick={() => handleDelete(u.id)}
                    disabled={me?.id === u.id}
                    title={me?.id === u.id ? "You can't delete yourself" : 'Delete user'}
                    className="px-3 py-1.5 bg-red-800 text-red-200 rounded text-sm hover:bg-red-700 disabled:opacity-40 disabled:cursor-not-allowed transition-colors"
                  >
                    Delete
                  </button>
                </div>
              </div>

              {/* Quota display */}
              <div className="mt-2 flex gap-4 text-xs text-gray-400">
                <span>Daily tokens: <span className="text-gray-300">{formatQuota(u.daily_token_limit)}</span></span>
                <span>Daily cost: <span className="text-gray-300">{formatQuota(u.daily_cost_limit, '$')}</span></span>
                <span>Monthly cost: <span className="text-gray-300">{formatQuota(u.monthly_cost_limit, '$')}</span></span>
              </div>

              {/* Quota editor */}
              {editingQuotaId === u.id && (
                <div className="mt-3 bg-gray-600 rounded p-3 space-y-2">
                  <p className="text-xs text-gray-300 mb-2">Set limits (0 = unlimited)</p>
                  <div className="grid grid-cols-3 gap-2">
                    <div>
                      <label className="block text-xs text-gray-400 mb-1">Daily Tokens</label>
                      <input
                        type="number"
                        min="0"
                        value={quotaDailyTokens}
                        onChange={(e) => setQuotaDailyTokens(e.target.value)}
                        className="w-full bg-gray-700 text-white rounded px-2 py-1 text-sm border border-gray-500 focus:outline-none focus:border-blue-500"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-gray-400 mb-1">Daily Cost ($)</label>
                      <input
                        type="number"
                        min="0"
                        step="0.01"
                        value={quotaDailyCost}
                        onChange={(e) => setQuotaDailyCost(e.target.value)}
                        className="w-full bg-gray-700 text-white rounded px-2 py-1 text-sm border border-gray-500 focus:outline-none focus:border-blue-500"
                      />
                    </div>
                    <div>
                      <label className="block text-xs text-gray-400 mb-1">Monthly Cost ($)</label>
                      <input
                        type="number"
                        min="0"
                        step="0.01"
                        value={quotaMonthlyCost}
                        onChange={(e) => setQuotaMonthlyCost(e.target.value)}
                        className="w-full bg-gray-700 text-white rounded px-2 py-1 text-sm border border-gray-500 focus:outline-none focus:border-blue-500"
                      />
                    </div>
                  </div>
                  <div className="flex gap-2 mt-2">
                    <button
                      onClick={() => handleSaveQuota(u.id)}
                      disabled={quotaSubmitting}
                      className="px-3 py-1.5 bg-blue-600 text-white rounded text-sm hover:bg-blue-500 disabled:opacity-50 transition-colors"
                    >
                      {quotaSubmitting ? 'Saving...' : 'Save'}
                    </button>
                    <button
                      onClick={() => setEditingQuotaId(null)}
                      className="px-3 py-1.5 bg-gray-500 text-white rounded text-sm hover:bg-gray-400 transition-colors"
                    >
                      Cancel
                    </button>
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
