import React from 'react';
import { Folder, Plus, Save, Loader2, Download } from 'lucide-react';

interface FileItem {
  id: number;
  name: string;
  updated_at?: string;
}

interface FileManagerProps {
  files: FileItem[];
  currentFileId: number | null;
  fileName: string;
  loading: boolean;
  saving: boolean;
  onNew: () => void;
  onSave: () => void;
  onSelect: (id: number) => void;
  onNameChange: (name: string) => void;
}

const FileManager: React.FC<FileManagerProps> = ({
  files,
  currentFileId,
  fileName,
  loading,
  saving,
  onNew,
  onSave,
  onSelect,
  onNameChange,
}) => {
  return (
    <div className="rounded-lg border border-gray-200 bg-white p-3 shadow-sm">
      <div className="flex items-center justify-between mb-2">
        <div className="flex items-center gap-2 text-sm font-semibold text-gray-800">
          <Folder size={16} />
          Fayllar
        </div>
        <button
          onClick={onNew}
          className="text-xs inline-flex items-center gap-1 px-2 py-1 rounded border border-gray-300 text-gray-700 hover:bg-gray-100"
        >
          <Plus size={14} /> Yangi
        </button>
      </div>

      <div className="flex items-center gap-2 mb-2">
        <input
          value={fileName}
          onChange={(e) => onNameChange(e.target.value)}
          placeholder="Fayl nomi"
          className="flex-1 border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-indigo-500"
        />
        <button
          onClick={onSave}
          disabled={saving || !fileName}
          className="inline-flex items-center gap-1 px-3 py-1.5 rounded text-white bg-indigo-600 hover:bg-indigo-700 text-sm font-medium disabled:opacity-60"
        >
          {saving ? <Loader2 size={16} className="animate-spin" /> : <Save size={16} />}
          Saqlash
        </button>
      </div>

      <div className="max-h-48 overflow-y-auto space-y-1 text-sm">
        {loading ? (
          <div className="text-gray-500 flex items-center gap-2">
            <Loader2 size={14} className="animate-spin" />
            Yuklanmoqda...
          </div>
        ) : files.length === 0 ? (
          <div className="text-gray-500">Fayllar yo‘q. Yangi fayl yarating.</div>
        ) : (
          files.map((f) => (
            <button
              key={f.id}
              onClick={() => onSelect(f.id)}
              className={`w-full flex items-center justify-between px-2 py-1 rounded border ${
                currentFileId === f.id ? 'border-indigo-400 bg-indigo-50 text-indigo-700' : 'border-transparent hover:border-gray-200'
              }`}
            >
              <span className="truncate">{f.name}</span>
              <Download size={14} />
            </button>
          ))
        )}
      </div>
    </div>
  );
};

export default FileManager;
