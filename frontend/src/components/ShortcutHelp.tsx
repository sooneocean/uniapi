interface Props {
  onClose: () => void;
}

const rows: { keys: string; desc: string }[] = [
  { keys: 'Ctrl/⌘ + K', desc: 'Search conversations' },
  { keys: 'Ctrl/⌘ + N', desc: 'New conversation' },
  { keys: 'Ctrl + /', desc: 'Focus message input' },
  { keys: 'Ctrl/⌘ + ,', desc: 'Open settings' },
  { keys: 'Ctrl + Shift + ?', desc: 'Show this help' },
  { keys: 'Enter', desc: 'Send message' },
  { keys: 'Shift + Enter', desc: 'New line' },
  { keys: 'Escape', desc: 'Close modal' },
];

export default function ShortcutHelp({ onClose }: Props) {
  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60"
      onClick={onClose}
    >
      <div
        className="rounded-xl border border-gray-600 shadow-2xl w-full max-w-sm mx-4"
        style={{ background: 'var(--bg-secondary)', color: 'var(--text-primary)' }}
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-700">
          <h2 className="font-semibold text-base">Keyboard Shortcuts</h2>
          <button
            onClick={onClose}
            className="text-gray-400 hover:text-white transition-colors text-lg leading-none"
            aria-label="Close"
          >
            &#10005;
          </button>
        </div>

        {/* Shortcut table */}
        <div className="px-5 py-4">
          <table className="w-full text-sm border-separate" style={{ borderSpacing: '0 6px' }}>
            <tbody>
              {rows.map(({ keys, desc }) => (
                <tr key={keys}>
                  <td className="pr-4 whitespace-nowrap">
                    <kbd className="font-mono text-xs bg-gray-700 border border-gray-600 rounded px-2 py-0.5 text-gray-200">
                      {keys}
                    </kbd>
                  </td>
                  <td className="text-gray-300">{desc}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>

        {/* Footer */}
        <div className="flex justify-end px-5 pb-4">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded-lg bg-gray-700 hover:bg-gray-600 text-gray-200 text-sm transition-colors border border-gray-600"
          >
            Close
          </button>
        </div>
      </div>
    </div>
  );
}
