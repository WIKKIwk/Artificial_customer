import { useEffect, useRef, useState } from 'react';

export type PresenceState = 'open' | 'closed';

export function usePresence(
  open: boolean,
  opts?: {
    exitDurationMs?: number;
  }
): { isMounted: boolean; state: PresenceState } {
  const exitDurationMs = opts?.exitDurationMs ?? 180;

  const [isMounted, setIsMounted] = useState(open);
  const [state, setState] = useState<PresenceState>(open ? 'closed' : 'closed');

  const timeoutIdRef = useRef<number | null>(null);
  const rafIdRef = useRef<number | null>(null);

  useEffect(() => {
    if (rafIdRef.current != null) {
      window.cancelAnimationFrame(rafIdRef.current);
      rafIdRef.current = null;
    }

    if (open) {
      if (timeoutIdRef.current != null) {
        window.clearTimeout(timeoutIdRef.current);
        timeoutIdRef.current = null;
      }
      setIsMounted(true);
      setState('closed');
      rafIdRef.current = window.requestAnimationFrame(() => {
        setState('open');
        rafIdRef.current = null;
      });
      return () => {
        if (rafIdRef.current != null) {
          window.cancelAnimationFrame(rafIdRef.current);
          rafIdRef.current = null;
        }
      };
    }

    return () => {
      if (timeoutIdRef.current != null) {
        window.clearTimeout(timeoutIdRef.current);
        timeoutIdRef.current = null;
      }
      if (rafIdRef.current != null) {
        window.cancelAnimationFrame(rafIdRef.current);
        rafIdRef.current = null;
      }
    };
  }, [open]);

  useEffect(() => {
    if (open) return;
    if (!isMounted) return;
    setState('closed');
    timeoutIdRef.current = window.setTimeout(() => {
      setIsMounted(false);
      timeoutIdRef.current = null;
    }, exitDurationMs);
    return () => {
      if (timeoutIdRef.current != null) {
        window.clearTimeout(timeoutIdRef.current);
        timeoutIdRef.current = null;
      }
    };
  }, [open, isMounted, exitDurationMs]);

  useEffect(() => {
    return () => {
      if (timeoutIdRef.current != null) {
        window.clearTimeout(timeoutIdRef.current);
        timeoutIdRef.current = null;
      }
      if (rafIdRef.current != null) {
        window.cancelAnimationFrame(rafIdRef.current);
        rafIdRef.current = null;
      }
    };
  }, []);

  return { isMounted, state };
}
