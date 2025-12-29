defmodule Converter.CRDT.Spreadsheet do
  @moduledoc """
  Ultra-low latency CRDT-based spreadsheet state manager.
  Handles thousands of concurrent users with conflict-free merging.
  """
  use GenServer
  require Logger

  @idle_timeout :timer.minutes(30)
  @persistence_interval :timer.seconds(10)

  defmodule State do
    defstruct [
      :spreadsheet_id,
      :crdt,
      :last_activity,
      :redis_key,
      :dirty
    ]
  end

  ## Client API

  def start_link(spreadsheet_id) do
    GenServer.start_link(__MODULE__, spreadsheet_id,
      name: via_tuple(spreadsheet_id)
    )
  end

  def get_state(spreadsheet_id) do
    GenServer.call(via_tuple(spreadsheet_id), :get_state, 15_000)
  end

  def update_cell(spreadsheet_id, row, col, value, user_id) do
    GenServer.cast(via_tuple(spreadsheet_id), {:update_cell, row, col, value, user_id})
  end

  def merge_remote_state(spreadsheet_id, remote_crdt) do
    GenServer.cast(via_tuple(spreadsheet_id), {:merge, remote_crdt})
  end

  ## Server Callbacks

  @impl true
  def init(spreadsheet_id) do
    redis_key = "spreadsheet:crdt:#{spreadsheet_id}"

    # Try to load from Redis first
    crdt =
      case load_from_redis(redis_key) do
        {:ok, saved_crdt} when is_map(saved_crdt) ->
          Logger.info("Loaded CRDT for #{spreadsheet_id} from Redis")
          saved_crdt

        _ ->
          Logger.info("Creating new CRDT for #{spreadsheet_id}")
          %{}
      end

    # Schedule periodic persistence
    Process.send_after(self(), :persist, @persistence_interval)

    # Schedule idle check
    Process.send_after(self(), :check_idle, @idle_timeout)

    {:ok, %State{
      spreadsheet_id: spreadsheet_id,
      crdt: crdt,
      last_activity: System.monotonic_time(:millisecond),
      redis_key: redis_key,
      dirty: false
    }}
  end

  @impl true
  def handle_call(:get_state, _from, state) do
    {:reply, {:ok, state.crdt}, %{state | last_activity: now()}}
  end

  @impl true
  def handle_cast({:update_cell, row, col, value, user_id}, state) do
    start_time = System.monotonic_time(:microsecond)

    # Create cell key
    cell_key = "#{row}:#{col}"

    # Update CRDT with cell value and metadata
    cell_data = %{
      value: value,
      user_id: user_id,
      timestamp: System.system_time(:millisecond)
    }

    new_crdt = Map.put(state.crdt, cell_key, cell_data)

    # Broadcast to other nodes in cluster
    Phoenix.PubSub.broadcast(
      Converter.PubSub,
      "crdt:#{state.spreadsheet_id}",
      {:crdt_update, new_crdt, cell_key, cell_data}
    )

    # Measure latency
    latency = System.monotonic_time(:microsecond) - start_time
    :telemetry.execute(
      [:converter, :spreadsheet, :edits, :latency],
      %{duration: latency},
      %{spreadsheet_id: state.spreadsheet_id}
    )

    :telemetry.execute(
      [:converter, :spreadsheet, :edits, :count],
      %{count: 1},
      %{}
    )

    {:noreply, %{state |
      crdt: new_crdt,
      last_activity: now(),
      dirty: true
    }}
  end

  @impl true
  def handle_cast({:merge, remote_crdt}, state) do
    merged_crdt =
      Map.merge(state.crdt, remote_crdt || %{}, fn _k, local, remote ->
        # Keep the one with higher timestamp
        if remote[:timestamp] && local[:timestamp] && remote.timestamp > local.timestamp do
          remote
        else
          local
        end
      end)

    :telemetry.execute(
      [:converter, :spreadsheet, :conflicts, :resolved],
      %{count: 1},
      %{}
    )

    {:noreply, %{state |
      crdt: merged_crdt,
      last_activity: now(),
      dirty: true
    }}
  end

  @impl true
  def handle_info(:persist, state) do
    if state.dirty do
      case save_to_redis(state.redis_key, state.crdt) do
        :ok ->
          Logger.debug("Persisted CRDT for #{state.spreadsheet_id}")
          Process.send_after(self(), :persist, @persistence_interval)
          {:noreply, %{state | dirty: false}}

        {:error, reason} ->
          Logger.error("Failed to persist CRDT: #{inspect(reason)}")
          Process.send_after(self(), :persist, @persistence_interval)
          {:noreply, state}
      end
    else
      Process.send_after(self(), :persist, @persistence_interval)
      {:noreply, state}
    end
  end

  @impl true
  def handle_info(:check_idle, state) do
    idle_time = now() - state.last_activity

    if idle_time > @idle_timeout do
      Logger.info("Stopping idle CRDT process for #{state.spreadsheet_id}")
      {:stop, :normal, state}
    else
      Process.send_after(self(), :check_idle, @idle_timeout)
      {:noreply, state}
    end
  end

  @impl true
  def terminate(reason, state) do
    Logger.info("Terminating CRDT for #{state.spreadsheet_id}: #{inspect(reason)}")

    # Final persist before shutdown
    if state.dirty do
      save_to_redis(state.redis_key, state.crdt)
    end

    :ok
  end

  ## Private Functions

  defp via_tuple(spreadsheet_id) do
    {:via, Registry, {Converter.CRDT.Registry, spreadsheet_id}}
  end

  defp load_from_redis(key) do
    case Redix.command(:redix, ["GET", key]) do
      {:ok, nil} ->
        {:error, :not_found}

      {:ok, binary} ->
        {:ok, :erlang.binary_to_term(binary)}

      error ->
        error
    end
  end

  defp save_to_redis(key, crdt) do
    binary = :erlang.term_to_binary(crdt)

    case Redix.command(:redix, ["SETEX", key, "7200", binary]) do
      {:ok, "OK"} -> :ok
      error -> error
    end
  end

  defp now, do: System.monotonic_time(:millisecond)
end
