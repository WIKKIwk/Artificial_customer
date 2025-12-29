import { Socket, Channel } from 'phoenix';
import { GridData } from '../types';
import { recomputeSheet, getCellId } from './spreadsheetUtils';

export type RealtimeHandlers = {
  onInitialState?: (data: GridData) => void;
  onCellUpdate?: (payload: { row: number; col: number; value: string; user_id: number }) => void;
  onBatchUpdate?: (edits: Array<{ row: number; col: number; value: string; user_id?: number }>) => void;
  onDisconnect?: () => void;
  onError?: (reason: string) => void;
};

const envRealtimeUrl = (import.meta.env.VITE_REALTIME_URL || '').trim();
const runtimeRealtimeUrl = (() => {
  if (typeof window === 'undefined') return 'ws://localhost:4000/socket';
  const scheme = window.location.protocol === 'https:' ? 'wss' : 'ws';
  return `${scheme}://${window.location.host}/socket`;
})();
const REALTIME_URL = envRealtimeUrl !== '' ? envRealtimeUrl : runtimeRealtimeUrl;

// Convert CRDT state from backend ("row:col" => %{value, ...}) to GridData ("row,col")
export const crdtStateToGrid = (state: Record<string, any>): GridData => {
  const grid: GridData = {};
  Object.entries(state || {}).forEach(([key, cell]: [string, any]) => {
    const [rowStr, colStr] = key.split(':');
    const row = parseInt(rowStr, 10);
    const col = parseInt(colStr, 10);
    if (Number.isNaN(row) || Number.isNaN(col)) return;
    grid[getCellId(row, col)] = { value: cell.value ?? '' };
  });
  return recomputeSheet(grid);
};

export class RealtimeClient {
  private socket: Socket | null = null;
  private channel: Channel | null = null;

  constructor(
    private opts: {
      token: string;
      sheetId: string | number;
      userName?: string;
      handlers?: RealtimeHandlers;
    }
  ) {}

  connect() {
    if (this.socket) return;

    this.socket = new Socket(REALTIME_URL, {
      params: { token: this.opts.token, user_name: this.opts.userName },
    });
    this.socket.connect();

    this.channel = this.socket.channel(`spreadsheet:${this.opts.sheetId}`, {
      token: this.opts.token,
      user_name: this.opts.userName,
    });

    this.channel.on('cell_update', (payload) => {
      this.opts.handlers?.onCellUpdate?.(payload);
    });

    this.channel.on('batch_update', ({ edits }) => {
      this.opts.handlers?.onBatchUpdate?.(edits || []);
    });

    this.channel.on('full_sync', ({ state }) => {
      const grid = crdtStateToGrid(state || {});
      this.opts.handlers?.onInitialState?.(grid);
    });

    this.channel.on('phx_error', () => this.opts.handlers?.onError?.('channel_error'));
    this.channel.on('phx_close', () => this.opts.handlers?.onDisconnect?.());

    this.channel
      .join()
      .receive('ok', ({ state }) => {
        if (state) {
          const grid = crdtStateToGrid(state);
          this.opts.handlers?.onInitialState?.(grid);
        }
      })
      .receive('error', (err) => {
        const reason = typeof err === 'string' ? err : err?.reason || 'join_error';
        this.opts.handlers?.onError?.(reason);
      });
  }

  disconnect() {
    if (this.channel) {
      this.channel.leave();
      this.channel = null;
    }
    if (this.socket) {
      this.socket.disconnect();
      this.socket = null;
    }
  }

  sendCellEdit(row: number, col: number, value: string) {
    this.channel?.push('cell_edit', { row, col, value });
  }

  sendBatch(edits: Array<{ row: number; col: number; value: string }>) {
    if (!this.channel) return;
    const maxBatchSize = 100;
    for (let i = 0; i < edits.length; i += maxBatchSize) {
      this.channel.push('batch_edit', { edits: edits.slice(i, i + maxBatchSize) });
    }
  }

  requestSync() {
    this.channel?.push('request_sync', {});
  }
}
