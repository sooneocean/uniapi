import { useState, useEffect } from 'react';

interface ThemeColors {
  primary: string;
  secondary: string;
  bg: string;
  surface: string;
  text: string;
  accent: string;
}

interface Theme {
  id: string;
  user_id: string;
  name: string;
  colors: string; // JSON
  shared: boolean;
  created_at: string;
}

const presets: Record<string, ThemeColors> = {
  'Default Dark': { primary: '#3b82f6', secondary: '#2563eb', bg: '#0f172a', surface: '#1e293b', text: '#f1f5f9', accent: '#60a5fa' },
  'Default Light': { primary: '#2563eb', secondary: '#1d4ed8', bg: '#f8fafc', surface: '#ffffff', text: '#0f172a', accent: '#3b82f6' },
  'Ocean': { primary: '#0ea5e9', secondary: '#0284c7', bg: '#0c1222', surface: '#162032', text: '#e2e8f0', accent: '#38bdf8' },
  'Forest': { primary: '#22c55e', secondary: '#16a34a', bg: '#0a1f0a', surface: '#142814', text: '#dcfce7', accent: '#4ade80' },
  'Sunset': { primary: '#f97316', secondary: '#ea580c', bg: '#1c0f05', surface: '#2d1a0b', text: '#fff7ed', accent: '#fb923c' },
  'Cyberpunk': { primary: '#a855f7', secondary: '#7c3aed', bg: '#0d0015', surface: '#1a0030', text: '#f3e8ff', accent: '#c084fc' },
};

const colorFields: { key: keyof ThemeColors; label: string }[] = [
  { key: 'primary', label: 'Primary' },
  { key: 'secondary', label: 'Secondary' },
  { key: 'bg', label: 'Background' },
  { key: 'surface', label: 'Surface' },
  { key: 'text', label: 'Text' },
  { key: 'accent', label: 'Accent' },
];

function applyTheme(colors: ThemeColors) {
  const root = document.documentElement;
  // Map to CSS custom properties used by the app
  root.style.setProperty('--color-primary', colors.primary);
  root.style.setProperty('--color-secondary', colors.secondary);
  root.style.setProperty('--color-bg', colors.bg);
  root.style.setProperty('--color-surface', colors.surface);
  root.style.setProperty('--color-text', colors.text);
  root.style.setProperty('--color-accent', colors.accent);
  // Also map to existing app CSS variables
  root.style.setProperty('--bg-primary', colors.bg);
  root.style.setProperty('--bg-secondary', colors.surface);
  root.style.setProperty('--bg-tertiary', colors.secondary);
  root.style.setProperty('--text-primary', colors.text);
  root.style.setProperty('--accent-color', colors.accent);
  localStorage.setItem('custom-theme', JSON.stringify(colors));
}

function loadSavedTheme() {
  try {
    const saved = localStorage.getItem('custom-theme');
    if (saved) applyTheme(JSON.parse(saved));
  } catch {}
}

// Call on module load to restore persisted theme
loadSavedTheme();

function getCSRF() {
  const m = document.cookie.match(/csrf_token=([^;]+)/);
  return m ? m[1] : '';
}

export default function ThemeEditor() {
  const [themes, setThemes] = useState<Theme[]>([]);
  const [colors, setColors] = useState<ThemeColors>(presets['Default Dark']);
  const [name, setName] = useState('My Theme');
  const [shared, setShared] = useState(false);
  const [saving, setSaving] = useState(false);
  const [activeId, setActiveId] = useState<string>('');

  const load = async () => {
    try {
      const resp = await fetch('/api/themes', { credentials: 'include' });
      if (resp.ok) setThemes(await resp.json());
    } catch {}
  };

  useEffect(() => { load(); }, []);

  const handleApply = (themeColors: ThemeColors, id?: string) => {
    applyTheme(themeColors);
    if (id) setActiveId(id);
    // Persist active theme to backend
    if (id) {
      fetch(`/api/themes/${id}/apply`, {
        method: 'PUT', credentials: 'include',
        headers: { 'X-CSRF-Token': getCSRF() },
      }).catch(() => {});
    }
  };

  const handleSave = async () => {
    setSaving(true);
    try {
      const resp = await fetch('/api/themes', {
        method: 'POST', credentials: 'include',
        headers: { 'Content-Type': 'application/json', 'X-CSRF-Token': getCSRF() },
        body: JSON.stringify({ name, colors: JSON.stringify(colors), shared }),
      });
      if (resp.ok) await load();
    } catch {}
    setSaving(false);
  };

  const handleDelete = async (id: string) => {
    await fetch(`/api/themes/${id}`, {
      method: 'DELETE', credentials: 'include',
      headers: { 'X-CSRF-Token': getCSRF() },
    });
    await load();
    if (activeId === id) setActiveId('');
  };

  const parseColors = (raw: string): ThemeColors => {
    try { return JSON.parse(raw); } catch { return presets['Default Dark']; }
  };

  const inputStyle = {
    background: 'var(--bg-primary)',
    color: 'var(--text-primary)',
    border: '1px solid var(--border-color)',
  };

  return (
    <div style={{ color: 'var(--text-primary)' }} className="space-y-6">
      <h2 className="text-lg font-semibold">Themes</h2>

      {/* Presets */}
      <div>
        <h3 className="text-sm font-semibold mb-2 uppercase tracking-wide" style={{ color: 'var(--text-secondary)' }}>Presets</h3>
        <div className="grid grid-cols-3 gap-2">
          {Object.entries(presets).map(([pName, pColors]) => (
            <button
              key={pName}
              onClick={() => { setColors(pColors); handleApply(pColors); }}
              className="rounded-lg p-2 text-xs text-left flex flex-col gap-1 transition-all"
              style={{ background: pColors.surface, border: `2px solid ${pColors.primary}` }}
            >
              <span style={{ color: pColors.text }} className="font-medium">{pName}</span>
              <div className="flex gap-1">
                {[pColors.primary, pColors.secondary, pColors.accent, pColors.text].map((c, i) => (
                  <div key={i} className="w-3 h-3 rounded-full" style={{ background: c }} />
                ))}
              </div>
            </button>
          ))}
        </div>
      </div>

      {/* Color editor */}
      <div>
        <h3 className="text-sm font-semibold mb-2 uppercase tracking-wide" style={{ color: 'var(--text-secondary)' }}>Customize</h3>
        <div className="grid grid-cols-2 gap-3 mb-3">
          {colorFields.map(({ key, label }) => (
            <div key={key} className="flex items-center gap-2">
              <input
                type="color"
                value={colors[key]}
                onChange={e => {
                  const next = { ...colors, [key]: e.target.value };
                  setColors(next);
                  applyTheme(next);
                }}
                className="w-8 h-8 rounded cursor-pointer border-0"
                style={{ padding: 2 }}
              />
              <div>
                <p className="text-xs font-medium">{label}</p>
                <p className="text-xs font-mono" style={{ color: 'var(--text-secondary)' }}>{colors[key]}</p>
              </div>
            </div>
          ))}
        </div>

        {/* Live preview */}
        <div className="rounded-lg p-3 mb-3" style={{ background: colors.bg, border: `1px solid ${colors.primary}` }}>
          <p className="text-xs font-semibold mb-1" style={{ color: colors.primary }}>Preview</p>
          <div className="rounded p-2" style={{ background: colors.surface }}>
            <p className="text-sm" style={{ color: colors.text }}>Sample text content</p>
            <div className="flex gap-2 mt-2">
              <button className="text-xs px-2 py-1 rounded" style={{ background: colors.primary, color: colors.bg }}>Primary</button>
              <button className="text-xs px-2 py-1 rounded" style={{ background: colors.accent, color: colors.bg }}>Accent</button>
            </div>
          </div>
        </div>

        <div className="flex gap-2 items-end">
          <div className="flex-1 space-y-1">
            <input
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="Theme name"
              className="w-full px-2 py-1.5 text-sm rounded"
              style={inputStyle}
            />
            <label className="flex items-center gap-1.5 text-xs" style={{ color: 'var(--text-secondary)' }}>
              <input type="checkbox" checked={shared} onChange={e => setShared(e.target.checked)} />
              Share with all users
            </label>
          </div>
          <button
            onClick={handleSave}
            disabled={saving || !name.trim()}
            className="px-3 py-1.5 text-sm rounded disabled:opacity-50"
            style={{ background: 'var(--accent-color)', color: 'white' }}
          >
            {saving ? 'Saving...' : 'Save Theme'}
          </button>
        </div>
      </div>

      {/* Saved themes */}
      {themes.length > 0 && (
        <div>
          <h3 className="text-sm font-semibold mb-2 uppercase tracking-wide" style={{ color: 'var(--text-secondary)' }}>Saved Themes</h3>
          <div className="space-y-1">
            {themes.map(t => {
              const tc = parseColors(t.colors);
              return (
                <div key={t.id} className="flex items-center gap-2 rounded-lg px-3 py-2" style={{ background: 'var(--bg-tertiary)', border: activeId === t.id ? '1px solid var(--accent-color)' : '1px solid transparent' }}>
                  <div className="flex gap-1">
                    {[tc.primary, tc.accent, tc.bg, tc.text].map((c, i) => (
                      <div key={i} className="w-3 h-3 rounded-full border" style={{ background: c, borderColor: 'var(--border-color)' }} />
                    ))}
                  </div>
                  <span className="flex-1 text-sm">{t.name}</span>
                  {t.shared && <span className="text-xs px-1 rounded" style={{ background: 'var(--bg-primary)', color: 'var(--text-secondary)' }}>shared</span>}
                  <button
                    onClick={() => { setColors(tc); handleApply(tc, t.id); }}
                    className="text-xs px-2 py-1 rounded"
                    style={{ background: activeId === t.id ? 'var(--accent-color)' : 'var(--bg-primary)', color: activeId === t.id ? 'white' : 'var(--text-primary)' }}
                  >
                    {activeId === t.id ? 'Active' : 'Apply'}
                  </button>
                  <button onClick={() => handleDelete(t.id)} className="text-xs text-red-400">✕</button>
                </div>
              );
            })}
          </div>
        </div>
      )}
    </div>
  );
}
