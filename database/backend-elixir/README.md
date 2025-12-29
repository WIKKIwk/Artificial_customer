# Elixir Real-time Backend

This is the "Brain" of the operation. It handles WebSocket connections for real-time collaboration.

## Prerequisites
- Elixir 1.14+
- Phoenix Framework

## Setup
1.  Install dependencies: `mix deps.get`
2.  Start the server: `mix phx.server`

## Architecture
- **SpreadsheetChannel**: Handles real-time edits. When User A sends an edit, it is broadcast to all other users in the same room.
