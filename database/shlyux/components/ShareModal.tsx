import React, { useEffect, useMemo, useRef, useState } from 'react';
import { X, Share2, Trash2, RefreshCw } from 'lucide-react';
import { FileShareRow, createFileShare, deleteFileShare, listFileShares } from '../utils/api';
import { usePresence } from '../utils/usePresence';

interface ShareModalProps {
  isOpen: boolean;
  fileId: number;
  fileName: string;
  token: string;
  onClose: () => void;
}

const ShareModal: React.FC<ShareModalProps> = ({ isOpen, fileId, fileName, token, onClose }) => {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [shares, setShares] = useState<FileShareRow[]>([]);

  const [email, setEmail] = useState('');
  const [role, setRole] = useState<'viewer' | 'editor'>('viewer');
  const [busyUserId, setBusyUserId] = useState<number | null>(null);
  const [adding, setAdding] = useState(false);

  const emailRef = useRef<HTMLInputElement | null>(null);
  const modalPresence = usePresence(isOpen, { exitDurationMs: 180 });

  const canSubmit = useMemo(() => {
    const trimmed = email.trim();
    return trimmed.length > 3 && trimmed.includes('@') && !adding;
  }, [email, adding]);

  const loadShares = async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await listFileShares(token, fileId);
      setShares(res.shares || []);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ulash ro'yxatini yuklab bo'lmadi");
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    if (!isOpen) return;
    loadShares();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isOpen, fileId]);

  useEffect(() => {
    if (!isOpen) return;
    const t = window.setTimeout(() => emailRef.current?.focus(), 0);
    return () => window.clearTimeout(t);
  }, [isOpen]);

  useEffect(() => {
    if (!isOpen) return;
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [isOpen, onClose]);

  const handleAdd = async () => {
    if (!canSubmit) return;
    setAdding(true);
    setError(null);
    try {
      await createFileShare(token, fileId, { email: email.trim(), role });
      setEmail('');
      setRole('viewer');
      await loadShares();
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Ulashda xato');
    } finally {
      setAdding(false);
    }
  };

  const handleDelete = async (row: FileShareRow) => {
    setBusyUserId(row.user_id);
    setError(null);
    try {
      await deleteFileShare(token, fileId, row.user_id);
      setShares(prev => prev.filter(s => s.user_id !== row.user_id));
    } catch (err) {
      setError(err instanceof Error ? err.message : "Ulashni o'chirishda xato");
    } finally {
      setBusyUserId(null);
    }
  };

  const handleRoleChange = async (row: FileShareRow, nextRole: 'viewer' | 'editor') => {
    setBusyUserId(row.user_id);
    setError(null);
    try {
      await createFileShare(token, fileId, { email: row.email, role: nextRole });
      setShares(prev => prev.map(s => (s.user_id === row.user_id ? { ...s, role: nextRole } : s)));
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Role yangilashda xato');
    } finally {
      setBusyUserId(null);
    }
  };

  if (!modalPresence.isMounted) return null;

  const roleLabel = (r: 'viewer' | 'editor') => (r === 'viewer' ? 'Ko‘rish (read-only)' : 'Tahrirlash (edit)');

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center ui-overlay"
      data-state={modalPresence.state}
      style={{ background: 'rgba(0,0,0,0.45)' }}
      onClick={onClose}
    >
      <div
        className="rounded-lg shadow-xl w-full max-w-2xl mx-4 ui-modal"
        data-state={modalPresence.state}
        onClick={(e) => e.stopPropagation()}
        style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)' }}
      >
        <div className="flex items-start justify-between p-6 border-b" style={{ borderColor: 'var(--border-color)' }}>
          <div className="flex items-center gap-3">
            <div
              className="w-10 h-10 rounded-full flex items-center justify-center"
              style={{ background: 'var(--sheet-selection-fill-weak)', border: '1px solid var(--border-color)' }}
            >
              <Share2 size={20} style={{ color: 'var(--brand)' }} />
            </div>
            <div>
              <h3 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>Ulash</h3>
              <p className="text-sm mt-1" style={{ color: 'var(--text-secondary)' }}>"{fileName}"</p>
            </div>
          </div>
          <button
            onClick={onClose}
            className="p-2 rounded hover:opacity-80 transition-opacity"
            style={{ color: 'var(--text-secondary)' }}
            title="Yopish"
          >
            <X size={20} />
          </button>
        </div>

        <div className="p-6 space-y-4">
          {error && (
            <div
              className="px-4 py-3 rounded border text-sm"
              style={{
                background: 'var(--sheet-danger-fill)',
                borderColor: 'var(--sheet-danger-border)',
                color: 'var(--text-primary)'
              }}
            >
              {error}
            </div>
          )}

          <div
            className="rounded-lg p-4"
            style={{ background: 'var(--bg-light)', border: '1px solid var(--border-color)' }}
          >
            <div className="grid grid-cols-1 md:grid-cols-[1fr_220px_auto] gap-3 items-end">
              <div>
                <label className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                  Email
                </label>
                <input
                  ref={emailRef}
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  className="mt-1 w-full px-3 py-2 border rounded-md text-sm focus:outline-none"
                  placeholder="user@example.com"
                  disabled={adding}
                  style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
                />
              </div>
              <div>
                <label className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
                  Ruxsat
                </label>
                <select
                  value={role}
                  onChange={(e) => setRole(e.target.value as 'viewer' | 'editor')}
                  className="mt-1 w-full px-3 py-2 border rounded-md text-sm focus:outline-none"
                  disabled={adding}
                  style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
                >
                  <option value="viewer">{roleLabel('viewer')}</option>
                  <option value="editor">{roleLabel('editor')}</option>
                </select>
              </div>
              <button
                onClick={handleAdd}
                disabled={!canSubmit}
                className="px-4 py-2 rounded-md text-sm font-medium text-white hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed btn-lift"
                style={{ background: 'var(--primary)' }}
              >
                {adding ? 'Qo‘shilmoqda…' : 'Qo‘shish'}
              </button>
            </div>

            <p className="mt-2 text-xs" style={{ color: 'var(--text-secondary)' }}>
              Viewer — faqat ko‘rish. Editor — tahrirlash va saqlash.
            </p>
          </div>

          <div className="flex items-center justify-between">
            <div className="text-sm font-medium" style={{ color: 'var(--text-primary)' }}>
              Ulashtirilgan foydalanuvchilar
            </div>
            <button
              onClick={loadShares}
              className="flex items-center gap-2 px-3 py-1.5 text-sm border rounded hover:opacity-80 transition-opacity"
              disabled={loading}
              title="Refresh"
              style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
            >
              <RefreshCw size={14} />
              Yangilash
            </button>
          </div>

          <div className="border rounded-md overflow-hidden" style={{ borderColor: 'var(--border-color)' }}>
            {loading ? (
              <div className="p-4 text-sm" style={{ color: 'var(--text-secondary)' }}>
                Yuklanmoqda…
              </div>
            ) : shares.length === 0 ? (
              <div className="p-4 text-sm" style={{ color: 'var(--text-secondary)' }}>
                Hali hech kimga ruxsat berilmagan.
              </div>
            ) : (
              <div className="divide-y">
                {shares.map((s) => (
                  <div key={s.user_id} className="flex items-center gap-3 p-3">
                    <div className="flex-1 min-w-0">
                      <div className="text-sm font-medium truncate" style={{ color: 'var(--text-primary)' }}>
                        {s.name || s.email}
                      </div>
                      <div className="text-xs truncate" style={{ color: 'var(--text-secondary)' }}>
                        {s.email}
                      </div>
                    </div>
                    <select
                      value={s.role}
                      onChange={(e) => handleRoleChange(s, e.target.value as 'viewer' | 'editor')}
                      className="px-3 py-2 border rounded-md text-sm"
                      disabled={busyUserId === s.user_id}
                      title="Role"
                      style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
                    >
                      <option value="viewer">{roleLabel('viewer')}</option>
                      <option value="editor">{roleLabel('editor')}</option>
                    </select>
                    <button
                      onClick={() => handleDelete(s)}
                      className="p-2 rounded hover:opacity-80 transition-opacity"
                      disabled={busyUserId === s.user_id}
                      title="Remove access"
                      style={{ color: '#ef4444' }}
                    >
                      <Trash2 size={16} />
                    </button>
                  </div>
                ))}
              </div>
            )}
          </div>
        </div>

        <div
          className="flex items-center justify-end gap-3 p-6 border-t rounded-b-lg"
          style={{ borderColor: 'var(--border-color)', background: 'var(--bg-light)' }}
        >
          <button
            onClick={onClose}
            className="px-4 py-2 text-sm font-medium border rounded-md hover:opacity-80 transition-opacity"
            style={{ borderColor: 'var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
          >
            Yopish
          </button>
        </div>
      </div>
    </div>
  );
};

export default ShareModal;
