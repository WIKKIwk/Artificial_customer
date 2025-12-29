defmodule Converter.CircuitBreaker do
  @moduledoc """
  Circuit breaker for Redis to prevent cascading failures.
  Automatically opens circuit when Redis is down, avoiding slowdowns.
  """
  use GenServer
  require Logger

  @failure_threshold 5        # Open after 5 failures
  @timeout 30_000             # Open for 30 seconds
  @half_open_attempts 3       # Test with 3 requests

  defmodule State do
    defstruct [
      :status,              # :closed | :open | :half_open
      :failure_count,
      :success_count,
      :last_failure_time,
      :timer
    ]
  end

  ## Client API

  def start_link(_) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @doc """
  Execute function with circuit breaker protection
  """
  def call(fun) when is_function(fun, 0) do
    GenServer.call(__MODULE__, {:execute, fun})
  end

  @doc """
  Get current circuit status
  """
  def status do
    GenServer.call(__MODULE__, :status)
  end

  ## Server Callbacks

  @impl true
  def init(_) do
    {:ok, %State{
      status: :closed,
      failure_count: 0,
      success_count: 0,
      last_failure_time: nil,
      timer: nil
    }}
  end

  @impl true
  def handle_call({:execute, fun}, _from, %{status: :open} = state) do
    # Circuit is open, fail fast
    Logger.debug("Circuit breaker is OPEN, failing fast")
    {:reply, {:error, :circuit_open}, state}
  end

  @impl true
  def handle_call({:execute, fun}, _from, %{status: :half_open} = state) do
    # Circuit is half-open, try the request
    case execute_with_monitoring(fun) do
      {:ok, result} ->
        new_state = record_success(state)
        {:reply, {:ok, result}, new_state}

      {:error, reason} ->
        new_state = record_failure(state)
        {:reply, {:error, reason}, new_state}
    end
  end

  @impl true
  def handle_call({:execute, fun}, _from, %{status: :closed} = state) do
    # Circuit is closed, execute normally
    case execute_with_monitoring(fun) do
      {:ok, result} ->
        {:reply, {:ok, result}, reset_failures(state)}

      {:error, reason} ->
        new_state = record_failure(state)
        {:reply, {:error, reason}, new_state}
    end
  end

  @impl true
  def handle_call(:status, _from, state) do
    {:reply, state.status, state}
  end

  @impl true
  def handle_info(:attempt_reset, state) do
    Logger.info("Circuit breaker attempting reset to HALF-OPEN")

    {:noreply, %{state |
      status: :half_open,
      success_count: 0,
      timer: nil
    }}
  end

  ## Private Functions

  defp execute_with_monitoring(fun) do
    start_time = System.monotonic_time(:microsecond)

    try do
      result = fun.()
      duration = System.monotonic_time(:microsecond) - start_time

      :telemetry.execute(
        [:converter, :circuit_breaker, :call],
        %{duration: duration},
        %{result: :success}
      )

      {:ok, result}
    rescue
      error ->
        duration = System.monotonic_time(:microsecond) - start_time

        :telemetry.execute(
          [:converter, :circuit_breaker, :call],
          %{duration: duration},
          %{result: :error}
        )

        {:error, error}
    end
  end

  defp record_failure(state) do
    new_failure_count = state.failure_count + 1
    new_state = %{state |
      failure_count: new_failure_count,
      last_failure_time: System.monotonic_time(:millisecond)
    }

    if new_failure_count >= @failure_threshold do
      open_circuit(new_state)
    else
      new_state
    end
  end

  defp record_success(%{status: :half_open} = state) do
    new_success_count = state.success_count + 1

    if new_success_count >= @half_open_attempts do
      Logger.info("Circuit breaker CLOSED after successful recovery")

      %{state |
        status: :closed,
        failure_count: 0,
        success_count: 0,
        timer: nil
      }
    else
      %{state | success_count: new_success_count}
    end
  end

  defp record_success(state) do
    reset_failures(state)
  end

  defp open_circuit(state) do
    Logger.error("Circuit breaker OPENED after #{state.failure_count} failures")

    # Cancel existing timer if any
    if state.timer, do: Process.cancel_timer(state.timer)

    # Schedule attempt to reset
    timer = Process.send_after(self(), :attempt_reset, @timeout)

    :telemetry.execute(
      [:converter, :circuit_breaker, :opened],
      %{count: 1},
      %{}
    )

    %{state |
      status: :open,
      timer: timer
    }
  end

  defp reset_failures(state) do
    %{state | failure_count: 0}
  end
end
