import { useState, useEffect } from 'react';
import { getAllUsage } from '../api/client';

type Range = 'daily' | 'weekly' | 'monthly';

interface UserUsage {
  username: string;
  total_cost: number;
  request_count: number;
}

interface ModelUsage {
  model: string;
  request_count: number;
  tokens_in: number;
  tokens_out: number;
  cost: number;
}

interface UsageData {
  users?: UserUsage[];
  models?: ModelUsage[];
  by_user?: UserUsage[];
  by_model?: ModelUsage[];
}

export default function UsageDashboard() {
  const [range, setRange] = useState<Range>('daily');
  const [data, setData] = useState<UsageData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = async (r: Range) => {
    try {
      setLoading(true);
      setError('');
      const result = await getAllUsage(r);
      setData(result);
    } catch {
      setError('Failed to load usage data');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(range); }, [range]);

  const userRows: UserUsage[] = data?.users ?? data?.by_user ?? [];
  const modelRows: ModelUsage[] = data?.models ?? data?.by_model ?? [];

  const maxCost = Math.max(...userRows.map((u) => u.total_cost), 0.0001);

  const exportCSV = () => {
    const lines: string[] = [];
    lines.push('Type,Name,Requests,Tokens In,Tokens Out,Cost');
    for (const u of userRows) {
      lines.push(`User,${u.username},${u.request_count},,,${u.total_cost.toFixed(4)}`);
    }
    for (const m of modelRows) {
      lines.push(`Model,${m.model},${m.request_count},${m.tokens_in},${m.tokens_out},${m.cost.toFixed(4)}`);
    }
    const csv = lines.join('\n');
    const blob = new Blob([csv], { type: 'text/csv' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = `usage-${range}-${new Date().toISOString().slice(0, 10)}.csv`;
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(url);
  };

  return (
    <div className="text-white">
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Usage</h2>
        <button
          onClick={exportCSV}
          className="px-3 py-1.5 bg-gray-600 text-gray-200 rounded-lg text-sm hover:bg-gray-500 transition-colors"
        >
          Export CSV
        </button>
      </div>

      {/* Range selector */}
      <div className="flex gap-1 mb-6">
        {(['daily', 'weekly', 'monthly'] as Range[]).map((r) => (
          <button
            key={r}
            onClick={() => setRange(r)}
            className={`px-4 py-1.5 rounded-lg text-sm font-medium transition-colors capitalize ${
              range === r
                ? 'bg-blue-600 text-white'
                : 'bg-gray-700 text-gray-300 hover:bg-gray-600'
            }`}
          >
            {r}
          </button>
        ))}
      </div>

      {error && <p className="text-red-400 text-sm mb-4">{error}</p>}

      {loading ? (
        <p className="text-gray-400 text-sm">Loading...</p>
      ) : (
        <>
          {/* Per-user cost breakdown */}
          <div className="mb-6">
            <h3 className="text-sm font-semibold text-gray-300 uppercase tracking-wide mb-3">Cost by User</h3>
            {userRows.length === 0 ? (
              <p className="text-gray-500 text-sm">No user data for this period.</p>
            ) : (
              <div className="space-y-2">
                {userRows.sort((a, b) => b.total_cost - a.total_cost).map((u) => (
                  <div key={u.username} className="flex items-center gap-3">
                    <span className="text-gray-300 text-sm w-32 truncate flex-shrink-0">{u.username}</span>
                    <div className="flex-1 bg-gray-700 rounded-full h-4 overflow-hidden">
                      <div
                        className="h-full bg-blue-500 rounded-full transition-all duration-300"
                        style={{ width: `${Math.min(100, (u.total_cost / maxCost) * 100)}%` }}
                      />
                    </div>
                    <span className="text-gray-300 text-sm w-20 text-right flex-shrink-0">
                      ${u.total_cost.toFixed(4)}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Per-model usage table */}
          <div>
            <h3 className="text-sm font-semibold text-gray-300 uppercase tracking-wide mb-3">Usage by Model</h3>
            {modelRows.length === 0 ? (
              <p className="text-gray-500 text-sm">No model data for this period.</p>
            ) : (
              <div className="overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="text-gray-400 border-b border-gray-700">
                      <th className="text-left py-2 pr-4">Model</th>
                      <th className="text-right py-2 px-4">Requests</th>
                      <th className="text-right py-2 px-4">Tokens In</th>
                      <th className="text-right py-2 px-4">Tokens Out</th>
                      <th className="text-right py-2 pl-4">Cost</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-gray-700">
                    {modelRows.sort((a, b) => b.cost - a.cost).map((m) => (
                      <tr key={m.model} className="text-gray-300 hover:bg-gray-700/50">
                        <td className="py-2 pr-4 font-mono text-xs">{m.model}</td>
                        <td className="text-right py-2 px-4">{m.request_count.toLocaleString()}</td>
                        <td className="text-right py-2 px-4">{m.tokens_in.toLocaleString()}</td>
                        <td className="text-right py-2 px-4">{m.tokens_out.toLocaleString()}</td>
                        <td className="text-right py-2 pl-4">${m.cost.toFixed(4)}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </>
      )}
    </div>
  );
}
