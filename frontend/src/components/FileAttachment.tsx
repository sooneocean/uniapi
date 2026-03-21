import { useRef } from 'react';

export interface AttachedFile {
  name: string;
  content: string;
  type: string;
}

const SUPPORTED_TYPES: Record<string, string> = {
  'text/plain': 'text',
  'text/markdown': 'text',
  'text/csv': 'text',
  'text/html': 'text',
  'application/json': 'text',
  'application/javascript': 'text',
  'text/javascript': 'text',
  'text/css': 'text',
  'text/xml': 'text',
  'application/xml': 'text',
  'application/x-yaml': 'text',
  'text/yaml': 'text',
};

async function readFileAsText(file: File): Promise<string> {
  return new Promise((resolve, reject) => {
    const reader = new FileReader();
    reader.onload = () => resolve(reader.result as string);
    reader.onerror = reject;
    reader.readAsText(file);
  });
}

function isCodeFile(name: string): boolean {
  const ext = name.split('.').pop()?.toLowerCase() || '';
  return ['py', 'js', 'ts', 'tsx', 'jsx', 'go', 'rs', 'java', 'c', 'cpp', 'h', 'hpp',
    'rb', 'php', 'swift', 'kt', 'scala', 'sh', 'bash', 'sql', 'r', 'lua',
    'toml', 'ini', 'cfg', 'env', 'dockerfile', 'makefile', 'yaml', 'yml',
    'md', 'txt', 'csv', 'log', 'xml', 'html', 'css', 'scss', 'json'].includes(ext);
}

const MAX_FILE_SIZE = 100 * 1024; // 100KB
const MAX_FILES = 5;

interface Props {
  attachedFiles: AttachedFile[];
  onFilesChange: (files: AttachedFile[]) => void;
  disabled?: boolean;
}

export default function FileAttachment({ attachedFiles, onFilesChange, disabled }: Props) {
  const fileInputRef = useRef<HTMLInputElement>(null);

  const processFiles = async (files: FileList | File[]) => {
    const fileArray = Array.from(files);
    const newFiles: AttachedFile[] = [];

    for (const file of fileArray) {
      if (attachedFiles.length + newFiles.length >= MAX_FILES) {
        alert(`Maximum ${MAX_FILES} files allowed`);
        break;
      }
      if (file.size > MAX_FILE_SIZE) {
        alert(`File "${file.name}" exceeds 100KB limit`);
        continue;
      }
      if (!SUPPORTED_TYPES[file.type] && !isCodeFile(file.name)) {
        alert(`File type not supported: "${file.name}"`);
        continue;
      }
      try {
        const content = await readFileAsText(file);
        newFiles.push({ name: file.name, content, type: file.type });
      } catch {
        alert(`Failed to read file: "${file.name}"`);
      }
    }

    if (newFiles.length > 0) {
      onFilesChange([...attachedFiles, ...newFiles]);
    }
  };

  const handleFileInputChange = async (e: React.ChangeEvent<HTMLInputElement>) => {
    const files = e.target.files;
    if (!files) return;
    await processFiles(files);
    e.target.value = '';
  };

  // Build accept string for file input
  const acceptTypes = [
    '.txt', '.md', '.csv', '.html', '.json', '.js', '.ts', '.tsx', '.jsx',
    '.css', '.scss', '.xml', '.yaml', '.yml', '.py', '.go', '.rs', '.java',
    '.c', '.cpp', '.h', '.hpp', '.rb', '.php', '.swift', '.kt', '.scala',
    '.sh', '.bash', '.sql', '.r', '.lua', '.toml', '.ini', '.cfg', '.env',
    '.log', '.dockerfile', '.makefile',
  ].join(',');

  return (
    <>
      <input
        ref={fileInputRef}
        type="file"
        accept={acceptTypes}
        multiple
        className="hidden"
        onChange={handleFileInputChange}
      />
      <button
        type="button"
        onClick={() => fileInputRef.current?.click()}
        disabled={disabled || attachedFiles.length >= MAX_FILES}
        className="px-3 py-3 bg-gray-700 text-gray-300 rounded-xl text-sm hover:bg-gray-600 disabled:opacity-50 disabled:cursor-not-allowed transition-colors flex-shrink-0"
        title="Attach file"
      >
        📄
      </button>
    </>
  );
}

export function formatAttachedFiles(files: AttachedFile[], userMessage: string): string {
  if (files.length === 0) return userMessage;

  const fileParts = files.map((file) => {
    const ext = file.name.split('.').pop()?.toLowerCase() || '';
    const lang = ext || 'text';
    return `[Attached: ${file.name}]\n\`\`\`${lang}\n${file.content}\n\`\`\``;
  });

  return fileParts.join('\n\n') + (userMessage ? '\n\n' + userMessage : '');
}
