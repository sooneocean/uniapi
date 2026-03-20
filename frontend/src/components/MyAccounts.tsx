import { useState, useEffect } from 'react';
import { getOAuthProviders, getOAuthAccounts, unbindAccount, reauthAccount } from '../api/client';
import SessionTokenDialog from './SessionTokenDialog';

interface OAuthProvider {
  name: string;
  display_name: string;
  supports_session_token: boolean;
  supports_oauth: boolean;
}

interface OAuthAccount {
  id: string;
  provider: string;
  label: string;
  auth_type: string;
  needs_reauth: boolean;
  owner_user_id: string;
}

interface Props {
  onBack: () => void;
}

export default function MyAccounts({ onBack }: Props) {
  const [providers, setProviders] = useState<OAuthProvider[]>([]);
  const [accounts, setAccounts] = useState<OAuthAccount[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');
  const [sessionDialog, setSessionDialog] = useState<{ provider: string; displayName: string } | null>(null);

  const load = async () => {
    try {
      setLoading(true);
      const [provData, accData] = await Promise.all([getOAuthProviders(), getOAuthAccounts()]);
      setProviders(Array.isArray(provData) ? provData : provData.providers ?? []);
      setAccounts(Array.isArray(accData) ? accData : accData.accounts ?? []);
    } catch {
      setError('Failed to load data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(); }, []);

  const handleUnbind = async (id: string) => {
    if (!confirm('Unbind this account?')) return;
    try {
      await unbindAccount(id);
      await load();
    } catch {
      setError('Failed to unbind account');
    }
  };

  const handleReauth = async (id: string) => {
    try {
      await reauthAccount(id);
      await load();
    } catch {
      setError('Failed to reauth account');
    }
  };

  const privateAccounts = accounts.filter((a) => a.owner_user_id !== '');
  const sharedAccounts = accounts.filter((a) => a.owner_user_id === '');

  return (
    <div className="min-h-screen bg-gray-900 text-white">
      {/* Header */}
      <div className="flex items-center gap-3 px-6 py-4 bg-gray-800 border-b border-gray-700">
        <button
          onClick={onBack}
          className="text-gray-400 hover:text-white transition-colors text-sm px-3 py-1.5 rounded hover:bg-gray-700 border border-gray-600"
        >
          ← Back
        </button>
        <h1 className="text-lg font-semibold">My Accounts</h1>
      </div>

      <div className="max-w-2xl mx-auto px-6 py-6 space-y-8">
        {error && <p className="text-red-400 text-sm">{error}</p>}

        {/* Bind AI Account section */}
        <section>
          <h2 className="text-base font-semibold mb-3 text-gray-200">Bind AI Account</h2>
          {loading ? (
            <p className="text-gray-400 text-sm">Loading providers...</p>
          ) : providers.length === 0 ? (
            <p className="text-gray-400 text-sm">No providers available.</p>
          ) : (
            <div className="space-y-3">
              {providers.map((prov) => (
                <div key={prov.name} className="bg-gray-800 rounded-lg p-4 flex items-center justify-between">
                  <span className="text-white font-medium">{prov.display_name}</span>
                  <div className="flex gap-2">
                    {prov.supports_session_token && (
                      <button
                        onClick={() => setSessionDialog({ provider: prov.name, displayName: prov.display_name })}
                        className="px-3 py-1.5 text-sm bg-blue-600 text-white rounded hover:bg-blue-500 transition-colors"
                      >
                        Paste Session Token
                      </button>
                    )}
                    {prov.supports_oauth && (
                      <button
                        onClick={() => window.open(`/api/oauth/bind/${prov.name}/authorize?shared=false`, '_blank', 'width=600,height=700')}
                        className="px-3 py-1.5 text-sm bg-green-700 text-white rounded hover:bg-green-600 transition-colors"
                      >
                        OAuth Connect
                      </button>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        {/* My Accounts section */}
        <section>
          <h2 className="text-base font-semibold mb-3 text-gray-200">My Accounts</h2>
          {privateAccounts.length === 0 ? (
            <p className="text-gray-400 text-sm">No private accounts bound.</p>
          ) : (
            <div className="space-y-3">
              {privateAccounts.map((acc) => (
                <div key={acc.id} className="bg-gray-800 rounded-lg p-4 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-white font-medium">{acc.label || acc.provider}</span>
                    <span className="text-xs bg-gray-600 text-gray-300 px-2 py-0.5 rounded">{acc.provider}</span>
                    <span className="text-xs bg-gray-700 text-gray-400 px-2 py-0.5 rounded">{acc.auth_type}</span>
                    {acc.needs_reauth ? (
                      <span className="text-xs bg-yellow-900 text-yellow-300 px-2 py-0.5 rounded">needs reauth</span>
                    ) : (
                      <span className="text-xs bg-green-900 text-green-300 px-2 py-0.5 rounded">normal</span>
                    )}
                  </div>
                  <div className="flex gap-2">
                    {acc.needs_reauth && (
                      <button
                        onClick={() => handleReauth(acc.id)}
                        className="px-3 py-1.5 text-sm bg-yellow-700 text-white rounded hover:bg-yellow-600 transition-colors"
                      >
                        Reauth
                      </button>
                    )}
                    <button
                      onClick={() => handleUnbind(acc.id)}
                      className="px-3 py-1.5 text-sm bg-red-800 text-red-200 rounded hover:bg-red-700 transition-colors"
                    >
                      Unbind
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>

        {/* Shared Accounts section */}
        <section>
          <h2 className="text-base font-semibold mb-3 text-gray-200">Shared Accounts</h2>
          {sharedAccounts.length === 0 ? (
            <p className="text-gray-400 text-sm">No shared accounts available.</p>
          ) : (
            <div className="space-y-3">
              {sharedAccounts.map((acc) => (
                <div key={acc.id} className="bg-gray-800 rounded-lg p-4 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <span className="text-white font-medium">{acc.label || acc.provider}</span>
                    <span className="text-xs bg-gray-600 text-gray-300 px-2 py-0.5 rounded">{acc.provider}</span>
                    {acc.needs_reauth ? (
                      <span className="text-xs bg-yellow-900 text-yellow-300 px-2 py-0.5 rounded">needs reauth</span>
                    ) : (
                      <span className="text-xs bg-green-900 text-green-300 px-2 py-0.5 rounded">normal</span>
                    )}
                  </div>
                </div>
              ))}
            </div>
          )}
        </section>
      </div>

      {sessionDialog && (
        <SessionTokenDialog
          provider={sessionDialog.provider}
          displayName={sessionDialog.displayName}
          shared={false}
          onClose={() => setSessionDialog(null)}
          onSuccess={() => { setSessionDialog(null); load(); }}
        />
      )}
    </div>
  );
}
