import { useState, useRef } from 'react';
import { exportAllData, importData } from '../api/client';

export default function DataSettings() {
  const [exporting, setExporting] = useState(false);
  const [importing, setImporting] = useState(false);
  const [importResult, setImportResult] = useState<{ imported_conversations: number } | null>(null);
  const [previewFile, setPreviewFile] = useState<File | null>(null);
  const [previewCount, setPreviewCount] = useState<number | null>(null);
  const [error, setError] = useState('');
  const fileRef = useRef<HTMLInputElement>(null);

  const handleExport = async () => {
    setExporting(true);
    setError('');
    try {
      await exportAllData();
    } catch {
      setError('Export failed');
    } finally {
      setExporting(false);
    }
  };

  const handleFileSelect = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const file = e.target.files?.[0];
    if (!file) return;
    setPreviewFile(file);
    setImportResult(null);
    setError('');
    try {
      const text = await file.text();
      const data = JSON.parse(text);
      const count = Array.isArray(data.conversations) ? data.conversations.length : 0;
      setPreviewCount(count);
    } catch {
      setError('Invalid JSON file');
      setPreviewFile(null);
      setPreviewCount(null);
    }
  };

  const handleImport = async () => {
    if (!previewFile) return;
    setImporting(true);
    setError('');
    try {
      const result = await importData(previewFile);
      setImportResult(result);
      setPreviewFile(null);
      setPreviewCount(null);
      if (fileRef.current) fileRef.current.value = '';
    } catch {
      setError('Import failed');
    } finally {
      setImporting(false);
    }
  };

  return (
    <div className="text-white space-y-6">
      <h2 className="text-lg font-semibold">Data Import / Export</h2>

      {error && <p className="text-red-400 text-sm">{error}</p>}

      {/* Export */}
      <div className="bg-gray-700 rounded-lg p-4">
        <h3 className="font-medium mb-1">Export All Data</h3>
        <p className="text-sm text-gray-400 mb-3">
          Download all your conversations, messages, and system prompts as a JSON file.
        </p>
        <button
          onClick={handleExport}
          disabled={exporting}
          className="px-4 py-2 bg-blue-600 text-white rounded-lg text-sm hover:bg-blue-500 disabled:opacity-50 transition-colors"
        >
          {exporting ? 'Exporting...' : 'Export All Data'}
        </button>
      </div>

      {/* Import */}
      <div className="bg-gray-700 rounded-lg p-4">
        <h3 className="font-medium mb-1">Import Data</h3>
        <p className="text-sm text-gray-400 mb-3">
          Import conversations from a previously exported UniAPI JSON file.
        </p>
        <input
          ref={fileRef}
          type="file"
          accept=".json"
          onChange={handleFileSelect}
          className="block w-full text-sm text-gray-300 file:mr-4 file:py-2 file:px-4 file:rounded file:border-0 file:text-sm file:bg-gray-600 file:text-white hover:file:bg-gray-500 mb-3"
        />

        {previewFile && previewCount !== null && (
          <div className="mb-3 p-3 bg-gray-600 rounded text-sm">
            <p className="text-gray-200">
              File: <span className="font-medium">{previewFile.name}</span>
            </p>
            <p className="text-gray-300 mt-1">
              Found <span className="font-medium text-white">{previewCount}</span> conversation{previewCount !== 1 ? 's' : ''} to import.
            </p>
          </div>
        )}

        {importResult && (
          <div className="mb-3 p-3 bg-green-900/40 border border-green-700 rounded text-sm text-green-300">
            Successfully imported {importResult.imported_conversations} conversation{importResult.imported_conversations !== 1 ? 's' : ''}.
          </div>
        )}

        <button
          onClick={handleImport}
          disabled={!previewFile || importing}
          className="px-4 py-2 bg-green-700 text-white rounded-lg text-sm hover:bg-green-600 disabled:opacity-50 transition-colors"
        >
          {importing ? 'Importing...' : 'Import'}
        </button>
      </div>
    </div>
  );
}
