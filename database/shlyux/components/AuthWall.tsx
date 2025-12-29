import React from 'react';
import { AlertCircle, LogIn, Sparkles, UserPlus } from 'lucide-react';
import { AuthUser } from '../utils/api';
import { usePresence } from '../utils/usePresence';

interface AuthWallProps {
  isOpen: boolean;
  user: AuthUser | null;
  authMode: 'login' | 'register';
  authForm: { name: string; email: string; password: string };
  authError: string | null;
  authLoading: boolean;
  onChangeAuthForm: (field: 'name' | 'email' | 'password', value: string) => void;
  onSubmitAuth: () => void;
  onToggleMode: () => void;
}

const AuthWall: React.FC<AuthWallProps> = ({
  isOpen,
  user,
  authMode,
  authForm,
  authError,
  authLoading,
  onChangeAuthForm,
  onSubmitAuth,
  onToggleMode,
}) => {
  const shouldOpen = isOpen && !user;
  const modalPresence = usePresence(shouldOpen, { exitDurationMs: 180 });

  if (!modalPresence.isMounted) return null;

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    onSubmitAuth();
  };

  return (
    <div
      className="fixed inset-0 z-40 flex items-center justify-center backdrop-blur-sm px-4 ui-overlay"
      data-state={modalPresence.state}
      style={{ background: 'rgba(0,0,0,0.5)' }}
    >
      <div
        className="w-full max-w-md rounded-lg shadow-2xl p-6 space-y-4 ui-modal"
        data-state={modalPresence.state}
        style={{ background: 'var(--card-bg)', border: '1px solid var(--border-color)' }}
      >
        <div className="flex items-center gap-2">
          <div className="p-2 rounded-lg" style={{ background: 'var(--bg-light)', color: 'var(--text-primary)' }}>
            <Sparkles size={18} />
          </div>
          <div>
            <h2 className="text-lg font-semibold" style={{ color: 'var(--text-primary)' }}>
              {authMode === 'register' ? "Ro'yxatdan o'ting" : 'Kirish'}
            </h2>
            <p className="text-sm" style={{ color: 'var(--text-secondary)' }}>
              Backend bilan ishlash uchun hisobga kiring.
            </p>
          </div>
        </div>

        {authError && (
          <div className="flex items-center gap-2 text-sm text-red-700 bg-red-50 border border-red-200 rounded-lg px-3 py-2">
            <AlertCircle size={16} />
            <span>{authError}</span>
          </div>
        )}

        <form className="space-y-2" onSubmit={handleSubmit}>
          {authMode === 'register' && (
            <input
              type="text"
              value={authForm.name}
              onChange={(e) => onChangeAuthForm('name', e.target.value)}
              placeholder="Ism"
              className="w-full rounded px-3 py-2 text-sm focus:outline-none"
              style={{ border: '1px solid var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
            />
          )}
          <input
            type="email"
            value={authForm.email}
            onChange={(e) => onChangeAuthForm('email', e.target.value)}
            placeholder="Email"
            className="w-full rounded px-3 py-2 text-sm focus:outline-none"
            style={{ border: '1px solid var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
          />
          <input
            type="password"
            value={authForm.password}
            onChange={(e) => onChangeAuthForm('password', e.target.value)}
            placeholder="Parol"
            className="w-full rounded px-3 py-2 text-sm focus:outline-none"
            style={{ border: '1px solid var(--border-color)', background: 'var(--card-bg)', color: 'var(--text-primary)' }}
          />
          <button
            onClick={onSubmitAuth}
            disabled={authLoading}
            className="w-full inline-flex items-center justify-center gap-2 text-white rounded py-2 text-sm font-medium hover:opacity-80 disabled:opacity-60 transition-opacity btn-lift"
            style={{ background: 'var(--primary)' }}
          >
            {authMode === 'register' ? <UserPlus size={16} /> : <LogIn size={16} />}
            {authMode === 'register' ? "Ro'yxatdan o'tish" : 'Kirish'}
          </button>
        </form>

        <div className="text-sm text-center" style={{ color: 'var(--text-secondary)' }}>
          {authMode === 'register' ? 'Hisobingiz bormi?' : "Hisob yo'qmi?"}{' '}
          <button
            onClick={onToggleMode}
            className="font-medium hover:opacity-80 transition-opacity"
            style={{ color: 'var(--text-primary)' }}
          >
            {authMode === 'register' ? "Kirishga o'tish" : "Ro'yxatdan o'tish"}
          </button>
        </div>
      </div>
    </div>
  );
};

export default AuthWall;
