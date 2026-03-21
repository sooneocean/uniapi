import { useState, useEffect } from 'react';
import {
  getSystemPrompts,
  createSystemPrompt,
  deleteSystemPrompt,
} from '../api/client';

interface SystemPrompt {
  id: string;
  name: string;
  content: string;
  is_default: boolean;
}

interface Props {
  value: string;
  onChange: (value: string) => void;
}

export default function SystemPromptSelector({ value, onChange }: Props) {
  const [presets, setPresets] = useState<SystemPrompt[]>([]);
  const [open, setOpen] = useState(false);
  const [selectedPresetId, setSelectedPresetId] = useState<string>('custom');
  const [customText, setCustomText] = useState('');
  const [showTextarea, setShowTextarea] = useState(false);
  const [savePresetName, setSavePresetName] = useState('');
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    getSystemPrompts()
      .then((data) => setPresets(data ?? []))
      .catch(() => {});
  }, []);

  // Sync external value into local state
  useEffect(() => {
    if (!value) {
      setSelectedPresetId('none');
      setCustomText('');
      setShowTextarea(false);
    }
  }, [value]);

  const handleSelectPreset = (id: string) => {
    setSelectedPresetId(id);
    if (id === 'none') {
      onChange('');
      setShowTextarea(false);
    } else if (id === 'custom') {
      onChange(customText);
      setShowTextarea(true);
    } else {
      const preset = presets.find((p) => p.id === id);
      if (preset) {
        onChange(preset.content);
        setShowTextarea(false);
      }
    }
    setOpen(false);
  };

  const handleCustomChange = (text: string) => {
    setCustomText(text);
    onChange(text);
  };

  const handleSavePreset = async () => {
    if (!savePresetName.trim() || !customText.trim()) return;
    setSaving(true);
    try {
      const sp = await createSystemPrompt({ name: savePresetName.trim(), content: customText.trim() });
      setPresets((prev) => [...prev, sp]);
      setSavePresetName('');
    } catch {
      // ignore
    } finally {
      setSaving(false);
    }
  };

  const handleDeletePreset = async (id: string, e: React.MouseEvent) => {
    e.stopPropagation();
    try {
      await deleteSystemPrompt(id);
      setPresets((prev) => prev.filter((p) => p.id !== id));
      if (selectedPresetId === id) {
        setSelectedPresetId('none');
        onChange('');
      }
    } catch {
      // ignore
    }
  };

  const selectedLabel =
    selectedPresetId === 'none'
      ? 'No system prompt'
      : selectedPresetId === 'custom'
      ? 'Custom'
      : presets.find((p) => p.id === selectedPresetId)?.name ?? 'Select...';

  const hasPrompt = value.trim().length > 0;

  return (
    <div className="relative">
      <button
        type="button"
        onClick={() => setOpen((v) => !v)}
        className={`flex items-center gap-1 px-2 py-1 rounded text-xs font-medium transition-colors border ${
          hasPrompt
            ? 'bg-purple-700 border-purple-500 text-white'
            : 'bg-gray-700 border-gray-600 text-gray-300 hover:bg-gray-600'
        }`}
        title="System prompt"
      >
        <span>⚙</span>
        <span>System</span>
        {hasPrompt && <span className="w-1.5 h-1.5 rounded-full bg-purple-300 inline-block ml-0.5" />}
      </button>

      {open && (
        <div
          className="absolute left-0 top-full mt-1 z-50 bg-gray-800 border border-gray-600 rounded-xl shadow-xl w-64"
          style={{ minWidth: '220px' }}
        >
          <div className="p-2 border-b border-gray-700">
            <p className="text-xs text-gray-400 font-medium px-1 mb-1">System Prompt</p>
            {/* No system prompt */}
            <button
              className={`w-full text-left px-2 py-1.5 rounded text-xs hover:bg-gray-700 transition-colors ${
                selectedPresetId === 'none' ? 'bg-gray-700 text-white' : 'text-gray-300'
              }`}
              onClick={() => handleSelectPreset('none')}
            >
              None
            </button>
            {/* Presets */}
            {presets.map((p) => (
              <div key={p.id} className="flex items-center group">
                <button
                  className={`flex-1 text-left px-2 py-1.5 rounded text-xs hover:bg-gray-700 transition-colors truncate ${
                    selectedPresetId === p.id ? 'bg-gray-700 text-white' : 'text-gray-300'
                  }`}
                  onClick={() => handleSelectPreset(p.id)}
                  title={p.name}
                >
                  {p.name}
                </button>
                <button
                  className="opacity-0 group-hover:opacity-100 text-gray-500 hover:text-red-400 px-1 text-xs transition-colors"
                  onClick={(e) => handleDeletePreset(p.id, e)}
                  title="Delete preset"
                >
                  ✕
                </button>
              </div>
            ))}
            {/* Custom option */}
            <button
              className={`w-full text-left px-2 py-1.5 rounded text-xs hover:bg-gray-700 transition-colors ${
                selectedPresetId === 'custom' ? 'bg-gray-700 text-white' : 'text-gray-300'
              }`}
              onClick={() => handleSelectPreset('custom')}
            >
              Custom...
            </button>
          </div>
          {/* Clear button */}
          {hasPrompt && (
            <div className="p-2 border-b border-gray-700">
              <button
                className="w-full px-2 py-1 rounded text-xs text-gray-400 hover:bg-gray-700 hover:text-gray-200 transition-colors"
                onClick={() => {
                  onChange('');
                  setSelectedPresetId('none');
                  setShowTextarea(false);
                  setOpen(false);
                }}
              >
                Clear prompt
              </button>
            </div>
          )}
          <div className="p-1">
            <button
              className="w-full text-left px-2 py-1 text-xs text-gray-400 hover:text-gray-200"
              onClick={() => setOpen(false)}
            >
              Close
            </button>
          </div>
        </div>
      )}

      {/* Active label if preset is set */}
      {selectedPresetId !== 'none' && selectedPresetId !== 'custom' && hasPrompt && (
        <p className="text-xs text-purple-400 mt-0.5 truncate max-w-[120px]">{selectedLabel}</p>
      )}

      {/* Custom textarea (visible when custom selected) */}
      {(showTextarea || selectedPresetId === 'custom') && (
        <div className="mt-2 w-full">
          <textarea
            value={customText}
            onChange={(e) => handleCustomChange(e.target.value)}
            placeholder="Enter system prompt..."
            rows={3}
            className="w-full bg-gray-700 text-gray-100 placeholder-gray-500 border border-gray-600 rounded-lg px-3 py-2 text-xs resize-none focus:outline-none focus:border-purple-500"
          />
          <div className="flex items-center gap-2 mt-1">
            <input
              type="text"
              value={savePresetName}
              onChange={(e) => setSavePresetName(e.target.value)}
              placeholder="Preset name..."
              className="flex-1 bg-gray-700 text-gray-200 placeholder-gray-500 border border-gray-600 rounded px-2 py-1 text-xs focus:outline-none focus:border-purple-500"
            />
            <button
              onClick={handleSavePreset}
              disabled={saving || !savePresetName.trim() || !customText.trim()}
              className="px-2 py-1 bg-purple-700 text-white rounded text-xs hover:bg-purple-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
            >
              Save
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
