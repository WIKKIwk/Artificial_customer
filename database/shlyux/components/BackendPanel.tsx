import React, { useRef } from 'react';
import { AlertCircle, CheckCircle2, LogIn, LogOut, Server, Upload } from 'lucide-react';
import { AuthUser } from '../utils/api';

interface BackendPanelProps {
  apiBase: string;
  user: AuthUser | null;
  token: string | null;
  authMode: 'login' | 'register';
  authForm: { name: string; email: string; password: string };
  authError: string | null;
  authLoading: boolean;
  converting: boolean;
  convertMessage: string | null;
  convertError: boolean;
  savedToDb: boolean;
  onChangeAuthForm: (field: 'name' | 'email' | 'password', value: string) => void;
  onSubmitAuth: () => void;
  onToggleMode: () => void;
  onLogout: () => void;
  onConvertFile: (file: File) => void;
}

const BackendPanel: React.FC<BackendPanelProps> = ({
  apiBase,
  user,
  token,
  authMode,
  authForm,
  authError,
  authLoading,
  converting,
  convertMessage,
  convertError,
  savedToDb,
  onChangeAuthForm,
  onSubmitAuth,
  onToggleMode,
  onLogout,
  onConvertFile,
}) => {
  const fileInputRef = useRef<HTMLInputElement>(null);
  const isAuthenticated = !!user;

  // Compact bar when logged in
  if (isAuthenticated && user) {
    return (
      <div className="bg-white border-b border-gray-200 px-4 py-2">
        <div className="flex flex-wrap items-center gap-2 text-sm">
          <span className="inline-flex items-center gap-1 px-2 py-1 rounded border border-gray-200 bg-gray-50">
            <Server size={14} className="text-blue-600" />
            <span className="text-gray-700">{apiBase}</span>
            <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-green-100 text-green-700">Auth</span>
          </span>
          <span className="inline-flex items-center gap-1 px-2 py-1 rounded border border-gray-200">
            <CheckCircle2 size={14} className="text-green-600" />
            <span className="font-medium text-gray-800">{user.name}</span>
            <span className="text-gray-500 text-xs">({user.email})</span>
          </span>
          <button
            onClick={onLogout}
            className="inline-flex items-center gap-1 px-2 py-1 rounded border border-gray-300 text-gray-700 hover:bg-gray-100 text-xs"
          >
            <LogOut size={14} />
            Logout
          </button>

          <div className="h-4 w-px bg-gray-200 mx-1" />

          <div className="inline-flex items-center gap-2">
            <span className="text-gray-700">Convert:</span>
            <input
              ref={fileInputRef}
              type="file"
              accept=".xlsx,.xls"
              className="text-xs"
              onChange={(e) => {
                const file = e.target.files?.[0];
                if (file) {
                  onConvertFile(file);
                  e.target.value = '';
                }
              }}
            />
            <button
              onClick={() => fileInputRef.current?.click()}
              disabled={converting}
              className="inline-flex items-center gap-1 px-3 py-1 rounded bg-indigo-600 text-white text-xs font-medium hover:bg-indigo-700 disabled:opacity-60"
            >
              <Upload size={14} />
              {converting ? '...' : 'Upload'}
            </button>
            {convertMessage && (
              <span className={`text-xs ${convertError ? 'text-red-600' : 'text-green-600'}`}>
                {convertMessage}
              </span>
            )}
            {!convertError && savedToDb && (
              <span className="text-[11px] px-2 py-0.5 rounded-full bg-green-50 text-green-700 border border-green-200">DB</span>
            )}
          </div>
        </div>
      </div>
    );
  }

  // Guest mode (keep forms but lighter padding)
  return (
    <div className="bg-white border-b border-gray-200 px-4 py-3">
      <div className="flex flex-wrap items-center gap-2 mb-3 text-sm text-gray-700">
        <Server size={14} className="text-blue-600" />
        <span className="font-medium text-gray-900">{apiBase}</span>
        <span className="px-2 py-0.5 rounded-full text-[11px] font-medium bg-orange-100 text-orange-700">Not authenticated</span>
      </div>
      <div className="max-w-xl rounded-lg border border-gray-200 bg-white p-3 shadow-sm">
        <div className="flex items-center justify-between mb-2">
          <div className="text-sm font-semibold text-gray-800">
            {authMode === 'register' ? 'Register & get token' : 'Login'}
          </div>
          <button
            onClick={onToggleMode}
            className="text-xs text-blue-600 hover:text-blue-700 font-medium"
          >
            {authMode === 'register' ? 'Use login instead' : 'Need an account? Register'}
          </button>
        </div>

        {authError && (
          <div className="flex items-center gap-2 text-xs text-red-700 bg-red-50 border border-red-200 rounded p-2 mb-2">
            <AlertCircle size={14} />
            <span>{authError}</span>
          </div>
        )}

        <div className="space-y-2">
          {authMode === 'register' && (
            <input
              type="text"
              value={authForm.name}
              onChange={(e) => onChangeAuthForm('name', e.target.value)}
              placeholder="Name"
              className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
          )}
          <input
            type="email"
            value={authForm.email}
            onChange={(e) => onChangeAuthForm('email', e.target.value)}
            placeholder="Email"
            className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <input
            type="password"
            value={authForm.password}
            onChange={(e) => onChangeAuthForm('password', e.target.value)}
            placeholder="Password"
            className="w-full border border-gray-300 rounded px-2 py-1.5 text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
          />
          <button
            onClick={onSubmitAuth}
            disabled={authLoading}
            className="w-full inline-flex items-center justify-center gap-2 bg-blue-600 text-white rounded py-1.5 text-sm font-medium hover:bg-blue-700 disabled:opacity-60"
          >
            {authLoading ? (
              'Processing...'
            ) : (
              <>
                <LogIn size={16} />
                {authMode === 'register' ? 'Register & Sign in' : 'Login'}
              </>
            )}
          </button>
        </div>
      </div>
    </div>
  );
};

export default BackendPanel;
