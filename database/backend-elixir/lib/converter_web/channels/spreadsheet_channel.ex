defmodule ConverterWeb.SpreadsheetChannel do
  @moduledoc """
  Ultra-low latency WebSocket channel for real-time spreadsheet collaboration.
  Optimized for thousands of concurrent users with sub-millisecond broadcast.
  """
  use ConverterWeb, :channel

  alias Converter.CRDT.Spreadsheet, as: SpreadsheetCRDT
  alias ConverterWeb.{AuthToken, Presence}
  require Logger

  @max_batch_size 100

  @impl true
  def join("spreadsheet:" <> spreadsheet_id, %{"token" => token} = payload, socket) do
    with {:ok, claims} <- AuthToken.verify_claims(token),
         {:ok, user_id} <- AuthToken.user_id_from_claims(claims),
         {:ok, sheet_id} <- sheet_id_from_claims(claims),
         true <- Integer.to_string(sheet_id) == spreadsheet_id do
      role = role_from_claims(claims)

      ensure_crdt_process(spreadsheet_id)
      {:ok, state} = SpreadsheetCRDT.get_state(spreadsheet_id)

      socket =
        socket
        |> assign(:spreadsheet_id, spreadsheet_id)
        |> assign(:user_id, user_id)
        |> assign(:user_name, payload["user_name"] || "User#{user_id}")
        |> assign(:role, role)

      Phoenix.PubSub.subscribe(Converter.PubSub, "crdt:#{spreadsheet_id}")
      send(self(), :after_join)

      {:ok, %{state: state, user_id: user_id, role: role}, socket}
    else
      _ -> {:error, %{reason: "unauthorized"}}
    end
  end

  @impl true
  def join("spreadsheet:" <> _spreadsheet_id, _payload, _socket) do
    {:error, %{reason: "missing_token"}}
  end

  @impl true
  def handle_info(:after_join, socket) do
    {:ok, _} =
      Presence.track(socket, socket.assigns.user_id, %{
        user_id: socket.assigns.user_id,
        user_name: socket.assigns.user_name,
        online_at: System.system_time(:second),
        color: generate_user_color(socket.assigns.user_id)
      })

    push(socket, "presence_state", Presence.list(socket))
    Logger.info("User #{socket.assigns.user_id} joined spreadsheet #{socket.assigns.spreadsheet_id}")

    {:noreply, socket}
  end

  # Handle CRDT updates from other nodes (cluster)
  @impl true
  def handle_info({:crdt_update, _crdt, cell_key, cell_data}, socket) do
    [row, col] = String.split(cell_key, ":")

    # Only broadcast if not from this socket's user
    if cell_data.user_id != socket.assigns.user_id do
      push(socket, "cell_update", %{
        row: String.to_integer(row),
        col: String.to_integer(col),
        value: cell_data.value,
        user_id: cell_data.user_id,
        timestamp: cell_data.timestamp
      })
    end

    {:noreply, socket}
  end

  # Handle cell edit from user
  @impl true
  def handle_in("cell_edit", %{"row" => row, "col" => col, "value" => value}, socket) do
    if socket.assigns[:role] == "viewer" do
      push(socket, "read_only", %{message: "Read-only access"})
      {:noreply, socket}
    else
      # Check rate limit
      case Converter.RateLimiter.check_rate(socket.assigns.user_id, 1) do
        {:ok, _remaining} ->
          start_time = System.monotonic_time(:microsecond)

          # Update CRDT
          SpreadsheetCRDT.update_cell(
            socket.assigns.spreadsheet_id,
            row,
            col,
            value,
            socket.assigns.user_id
          )

          # Broadcast to all users (including sender for confirmation)
          broadcast!(socket, "cell_update", %{
            row: row,
            col: col,
            value: value,
            user_id: socket.assigns.user_id,
            timestamp: System.system_time(:millisecond)
          })

          # Measure end-to-end latency
          latency = System.monotonic_time(:microsecond) - start_time
          if latency > 1000 do
            Logger.warn("High latency detected: #{latency}μs for user #{socket.assigns.user_id}")
          end

          {:noreply, socket}

        {:error, :rate_limited} ->
          # User exceeded rate limit
          push(socket, "rate_limited", %{
            message: "Too many requests. Please slow down.",
            retry_after: 1000
          })

          {:noreply, socket}
      end
    end
  end

  # Batch edits for performance (e.g., paste operation)
  @impl true
  def handle_in("batch_edit", %{"edits" => edits}, socket) when is_list(edits) do
    if socket.assigns[:role] == "viewer" do
      push(socket, "read_only", %{message: "Read-only access"})
      {:noreply, socket}
    else
      # Limit batch size
      edits = Enum.take(edits, @max_batch_size)

      Enum.each(edits, fn %{"row" => row, "col" => col, "value" => value} ->
        SpreadsheetCRDT.update_cell(
          socket.assigns.spreadsheet_id,
          row,
          col,
          value,
          socket.assigns.user_id
        )
      end)

      broadcast!(socket, "batch_update", %{
        edits: edits,
        user_id: socket.assigns.user_id,
        timestamp: System.system_time(:millisecond)
      })

      {:noreply, socket}
    end
  end

  # Handle cursor movement for collaborative cursors
  @impl true
  def handle_in("cursor_move", %{"row" => row, "col" => col}, socket) do
    broadcast_from!(socket, "cursor_update", %{
      user_id: socket.assigns.user_id,
      user_name: socket.assigns.user_name,
      row: row,
      col: col
    })

    {:noreply, socket}
  end

  # Request full state sync (in case of desync)
  @impl true
  def handle_in("request_sync", _payload, socket) do
    {:ok, state} = SpreadsheetCRDT.get_state(socket.assigns.spreadsheet_id)

    push(socket, "full_sync", %{
      state: state,
      timestamp: System.system_time(:millisecond)
    })

    {:noreply, socket}
  end

  ## Private Functions

  defp ensure_crdt_process(spreadsheet_id) do
    case Registry.lookup(Converter.CRDT.Registry, spreadsheet_id) do
      [] ->
        # Start new CRDT process
        {:ok, _pid} =
          DynamicSupervisor.start_child(
            Converter.CRDT.SpreadsheetSupervisor,
            {SpreadsheetCRDT, spreadsheet_id}
          )

      [{_pid, _}] ->
        # Already exists
        :ok
    end
  end

  defp sheet_id_from_claims(%{"sheet_id" => sheet_id}) when is_integer(sheet_id), do: {:ok, sheet_id}

  defp sheet_id_from_claims(%{"sheet_id" => sheet_id}) when is_binary(sheet_id) do
    case Integer.parse(sheet_id) do
      {val, _} -> {:ok, val}
      :error -> {:error, :invalid_sheet_id}
    end
  end

  defp sheet_id_from_claims(_), do: {:error, :missing_sheet_id}

  defp role_from_claims(%{"role" => role}) when is_binary(role) do
    role = role |> String.trim() |> String.downcase()
    if role in ["owner", "editor", "viewer"], do: role, else: "viewer"
  end

  defp role_from_claims(_), do: "viewer"

  defp generate_user_color(user_id) do
    # Generate consistent color for user
    colors = [
      "#FF6B6B",
      "#4ECDC4",
      "#45B7D1",
      "#FFA07A",
      "#98D8C8",
      "#F7DC6F",
      "#BB8FCE",
      "#85C1E2"
    ]

    Enum.at(colors, rem(user_id, length(colors)))
  end
end
