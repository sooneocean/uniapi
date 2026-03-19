import { useEffect, useState } from 'react';
import type { ModelInfo } from '../types';
import { fetchModels } from '../api/client';

interface Props {
  selectedModel: string;
  onModelChange: (model: string) => void;
}

export default function ModelSelector({ selectedModel, onModelChange }: Props) {
  const [models, setModels] = useState<ModelInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    fetchModels()
      .then((data) => {
        setModels(data);
        if (data.length > 0 && !selectedModel) {
          onModelChange(data[0].id);
        }
      })
      .catch(() => setError('Failed to load models'))
      .finally(() => setLoading(false));
  }, []);

  if (loading) {
    return (
      <div className="text-gray-400 text-sm px-3 py-2">Loading models...</div>
    );
  }

  if (error) {
    return (
      <div className="text-red-400 text-sm px-3 py-2">{error}</div>
    );
  }

  return (
    <select
      value={selectedModel}
      onChange={(e) => onModelChange(e.target.value)}
      className="bg-gray-700 text-gray-100 border border-gray-600 rounded-lg px-3 py-2 text-sm focus:outline-none focus:border-blue-500 cursor-pointer"
    >
      {models.length === 0 && (
        <option value="">No models available</option>
      )}
      {models.map((m) => (
        <option key={m.id} value={m.id}>
          {m.id}
        </option>
      ))}
    </select>
  );
}
