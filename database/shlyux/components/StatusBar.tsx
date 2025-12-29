import React, { useMemo } from 'react';

type Tone = 'neutral' | 'success' | 'warning' | 'danger';

const toneStyles = (tone: Tone): React.CSSProperties => {
  switch (tone) {
    case 'success':
      return { background: 'rgba(34, 197, 94, 0.12)', borderColor: 'rgba(34, 197, 94, 0.35)', color: 'rgba(34, 197, 94, 0.95)' };
    case 'warning':
      return { background: 'rgba(245, 158, 11, 0.12)', borderColor: 'rgba(245, 158, 11, 0.35)', color: 'rgba(245, 158, 11, 0.95)' };
    case 'danger':
      return { background: 'rgba(239, 68, 68, 0.12)', borderColor: 'rgba(239, 68, 68, 0.35)', color: 'rgba(239, 68, 68, 0.95)' };
    default:
      return { background: 'var(--bg-light)', borderColor: 'var(--border-color)', color: 'var(--text-secondary)' };
  }
};

type StatusBarProps = {
  density?: 'comfortable' | 'compact';
  activeCellLabel: string;
  selectionLabel?: string;
  accessRole: 'owner' | 'editor' | 'viewer';
  autoSaveEnabled: boolean;
  saveStatus: 'idle' | 'saving' | 'saved' | 'error';
  lastSavedAt: number | null;
  realtimeStatus: 'disconnected' | 'connecting' | 'connected' | 'error';
};

const StatusBar: React.FC<StatusBarProps> = ({
  density,
  activeCellLabel,
  selectionLabel,
  accessRole,
  autoSaveEnabled,
  saveStatus,
  lastSavedAt,
  realtimeStatus,
}) => {
  const height = density === 'compact' ? 28 : 32;

  const savedTime = useMemo(() => {
    if (!lastSavedAt) return null;
    try {
      return new Date(lastSavedAt).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' });
    } catch {
      return null;
    }
  }, [lastSavedAt]);

  const savePill = useMemo(() => {
    const prefix = autoSaveEnabled ? 'Auto-save' : 'Save';
    switch (saveStatus) {
      case 'saving':
        return { tone: 'warning' as const, text: `${prefix}: saving…` };
      case 'saved':
        return { tone: 'success' as const, text: savedTime ? `${prefix}: saved ${savedTime}` : `${prefix}: saved` };
      case 'error':
        return { tone: 'danger' as const, text: `${prefix}: error` };
      default:
        return { tone: 'neutral' as const, text: `${prefix}: idle` };
    }
  }, [autoSaveEnabled, saveStatus, savedTime]);

  const realtimePill = useMemo(() => {
    switch (realtimeStatus) {
      case 'connected':
        return { tone: 'success' as const, text: 'Realtime: connected' };
      case 'connecting':
        return { tone: 'warning' as const, text: 'Realtime: connecting…' };
      case 'error':
        return { tone: 'danger' as const, text: 'Realtime: error' };
      default:
        return { tone: 'neutral' as const, text: 'Realtime: off' };
    }
  }, [realtimeStatus]);

  const rolePill = useMemo(() => {
    if (accessRole === 'viewer') return { tone: 'neutral' as const, text: 'Role: read-only' };
    if (accessRole === 'editor') return { tone: 'neutral' as const, text: 'Role: editor' };
    return { tone: 'neutral' as const, text: 'Role: owner' };
  }, [accessRole]);

  return (
    <div
      className="flex items-center justify-between px-3 text-xs"
      style={{
        height,
        background: 'var(--card-bg)',
        borderTop: '1px solid var(--border-color)',
        gap: 12,
      }}
    >
      <div className="flex items-center gap-3 min-w-0">
        <span style={{ color: 'var(--text-primary)', fontWeight: 600 }}>{activeCellLabel || '-'}</span>
        {selectionLabel && (
          <span className="truncate" style={{ color: 'var(--text-secondary)' }}>
            {selectionLabel}
          </span>
        )}
      </div>

      <div className="flex items-center gap-2 flex-shrink-0">
        {[realtimePill, savePill, rolePill].map((pill) => (
          <span
            key={pill.text}
            className="px-2 py-0.5 rounded border"
            style={{
              ...toneStyles(pill.tone),
              borderWidth: 1,
              borderStyle: 'solid',
              fontWeight: 600,
              letterSpacing: '0.1px',
            }}
          >
            {pill.text}
          </span>
        ))}
      </div>
    </div>
  );
};

export default StatusBar;

