import { useState, useEffect } from 'react';
import {
  LineChart, Line, BarChart, Bar, PieChart, Pie, Cell, AreaChart, Area,
  XAxis, YAxis, CartesianGrid, Tooltip, Legend, ResponsiveContainer,
} from 'recharts';

interface DailyEntry {
  date: string;
  cost: number;
  tokens_in: number;
  tokens_out: number;
  request_count: number;
}

interface ModelEntry {
  model: string;
  cost: number;
}

interface UserEntry {
  username: string;
  cost: number;
  request_count: number;
}

interface AnalyticsData {
  daily: DailyEntry[];
  by_model: ModelEntry[];
  top_users: UserEntry[] | null;
}

const COLORS = ['#3b82f6', '#10b981', '#f59e0b', '#ef4444', '#8b5cf6', '#06b6d4', '#f97316', '#84cc16'];

const shortDate = (d: string) => d.slice(5); // MM-DD
const fmt = (n: number) => n.toFixed(4);
const fmtK = (n: number) => n >= 1000 ? `${(n / 1000).toFixed(1)}k` : String(n);

export default function AnalyticsDashboard() {
  const [days, setDays] = useState(30);
  const [data, setData] = useState<AnalyticsData | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState('');

  const load = async (d: number) => {
    setLoading(true);
    setError('');
    try {
      const resp = await fetch(`/api/usage/analytics?days=${d}`, { credentials: 'include' });
      if (!resp.ok) throw new Error('Failed to load analytics');
      setData(await resp.json());
    } catch (e: any) {
      setError(e.message || 'Failed to load analytics');
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { load(days); }, [days]);

  const labelStyle = { color: 'var(--text-secondary)', fontSize: 12 };
  const axisStyle = { fill: 'var(--text-secondary)', fontSize: 11 };
  const gridColor = 'rgba(255,255,255,0.08)';
  const tooltipStyle = {
    backgroundColor: 'var(--bg-secondary)',
    border: '1px solid var(--border-color)',
    color: 'var(--text-primary)',
    fontSize: 12,
    borderRadius: 6,
  };

  return (
    <div style={{ color: 'var(--text-primary)' }}>
      <div className="flex items-center justify-between mb-4">
        <h2 className="text-lg font-semibold">Usage Analytics</h2>
        <div className="flex gap-1">
          {[7, 14, 30, 60, 90].map(d => (
            <button
              key={d}
              onClick={() => setDays(d)}
              className="px-3 py-1 text-sm rounded"
              style={{
                background: days === d ? 'var(--accent-color)' : 'var(--bg-tertiary)',
                color: days === d ? 'white' : 'var(--text-secondary)',
              }}
            >{d}d</button>
          ))}
        </div>
      </div>

      {error && <p className="text-red-400 text-sm mb-4">{error}</p>}
      {loading ? (
        <p style={{ color: 'var(--text-secondary)' }} className="text-sm">Loading analytics...</p>
      ) : data && (
        <div className="space-y-6">
          {/* Cost over time */}
          <div>
            <h3 className="text-sm font-semibold mb-2 uppercase tracking-wide" style={labelStyle}>Cost Over Time</h3>
            <ResponsiveContainer width="100%" height={180}>
              <LineChart data={data.daily}>
                <CartesianGrid strokeDasharray="3 3" stroke={gridColor} />
                <XAxis dataKey="date" tickFormatter={shortDate} tick={axisStyle} />
                <YAxis tickFormatter={v => `$${fmt(v)}`} tick={axisStyle} width={60} />
                <Tooltip contentStyle={tooltipStyle} formatter={(v: any) => [`$${fmt(v)}`, 'Cost']} />
                <Line type="monotone" dataKey="cost" stroke="#3b82f6" strokeWidth={2} dot={false} />
              </LineChart>
            </ResponsiveContainer>
          </div>

          {/* Token usage bar chart */}
          <div>
            <h3 className="text-sm font-semibold mb-2 uppercase tracking-wide" style={labelStyle}>Token Usage</h3>
            <ResponsiveContainer width="100%" height={180}>
              <BarChart data={data.daily}>
                <CartesianGrid strokeDasharray="3 3" stroke={gridColor} />
                <XAxis dataKey="date" tickFormatter={shortDate} tick={axisStyle} />
                <YAxis tickFormatter={fmtK} tick={axisStyle} width={48} />
                <Tooltip contentStyle={tooltipStyle} formatter={(v: any) => [v.toLocaleString(), '']} />
                <Legend wrapperStyle={{ fontSize: 11 }} />
                <Bar dataKey="tokens_in" name="Input Tokens" fill="#10b981" stackId="a" />
                <Bar dataKey="tokens_out" name="Output Tokens" fill="#3b82f6" stackId="a" />
              </BarChart>
            </ResponsiveContainer>
          </div>

          {/* Request volume */}
          <div>
            <h3 className="text-sm font-semibold mb-2 uppercase tracking-wide" style={labelStyle}>Request Volume</h3>
            <ResponsiveContainer width="100%" height={150}>
              <AreaChart data={data.daily}>
                <CartesianGrid strokeDasharray="3 3" stroke={gridColor} />
                <XAxis dataKey="date" tickFormatter={shortDate} tick={axisStyle} />
                <YAxis tick={axisStyle} width={36} />
                <Tooltip contentStyle={tooltipStyle} formatter={(v: any) => [v, 'Requests']} />
                <Area type="monotone" dataKey="request_count" name="Requests" stroke="#f59e0b" fill="rgba(245,158,11,0.15)" />
              </AreaChart>
            </ResponsiveContainer>
          </div>

          {/* Cost by model — pie chart */}
          {data.by_model && data.by_model.length > 0 && (
            <div>
              <h3 className="text-sm font-semibold mb-2 uppercase tracking-wide" style={labelStyle}>Cost by Model</h3>
              <div className="flex items-center gap-4">
                <ResponsiveContainer width={180} height={180}>
                  <PieChart>
                    <Pie data={data.by_model} dataKey="cost" nameKey="model" cx="50%" cy="50%" outerRadius={75} innerRadius={35}>
                      {data.by_model.map((_, i) => (
                        <Cell key={i} fill={COLORS[i % COLORS.length]} />
                      ))}
                    </Pie>
                    <Tooltip contentStyle={tooltipStyle} formatter={(v: any) => [`$${fmt(v)}`, 'Cost']} />
                  </PieChart>
                </ResponsiveContainer>
                <div className="flex-1 space-y-1">
                  {data.by_model.slice(0, 8).map((m, i) => (
                    <div key={i} className="flex items-center gap-2 text-xs">
                      <div className="w-2.5 h-2.5 rounded-sm flex-shrink-0" style={{ background: COLORS[i % COLORS.length] }} />
                      <span className="truncate flex-1 font-mono" style={{ color: 'var(--text-primary)' }}>{m.model}</span>
                      <span style={{ color: 'var(--text-secondary)' }}>${fmt(m.cost)}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          )}

          {/* Top users (admin only) */}
          {data.top_users && data.top_users.length > 0 && (
            <div>
              <h3 className="text-sm font-semibold mb-2 uppercase tracking-wide" style={labelStyle}>Top Users</h3>
              <ResponsiveContainer width="100%" height={Math.min(200, data.top_users.length * 28 + 20)}>
                <BarChart data={data.top_users} layout="vertical">
                  <CartesianGrid strokeDasharray="3 3" stroke={gridColor} />
                  <XAxis type="number" tickFormatter={v => `$${fmt(v)}`} tick={axisStyle} />
                  <YAxis type="category" dataKey="username" tick={axisStyle} width={80} />
                  <Tooltip contentStyle={tooltipStyle} formatter={(v: any) => [`$${fmt(v)}`, 'Cost']} />
                  <Bar dataKey="cost" fill="#8b5cf6" />
                </BarChart>
              </ResponsiveContainer>
            </div>
          )}

          {data.daily.length === 0 && data.by_model.length === 0 && (
            <p className="text-sm text-center py-8" style={{ color: 'var(--text-secondary)' }}>No usage data for this period.</p>
          )}
        </div>
      )}
    </div>
  );
}
