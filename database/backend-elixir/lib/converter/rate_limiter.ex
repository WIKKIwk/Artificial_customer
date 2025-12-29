defmodule Converter.RateLimiter do
  @moduledoc """
  Token bucket rate limiter to prevent server overload.
  Limits edits per user to protect against DoS attacks.
  """
  use GenServer
  require Logger

  @max_tokens 100        # Max burst
  @refill_rate 20        # Tokens per second
  @refill_interval 100   # Refill every 100ms

  defmodule State do
    defstruct users: %{}, cleanup_timer: nil
  end

  ## Client API

  def start_link(_) do
    GenServer.start_link(__MODULE__, [], name: __MODULE__)
  end

  @doc """
  Check if user can perform action. Returns {:ok, remaining} or {:error, :rate_limited}
  """
  def check_rate(user_id, cost \\ 1) do
    GenServer.call(__MODULE__, {:check_rate, user_id, cost})
  end

  @doc """
  Get current rate limit status for user
  """
  def get_status(user_id) do
    GenServer.call(__MODULE__, {:get_status, user_id})
  end

  ## Server Callbacks

  @impl true
  def init(_) do
    # Schedule token refill
    Process.send_after(self(), :refill_tokens, @refill_interval)

    # Schedule cleanup of inactive users
    cleanup_timer = Process.send_after(self(), :cleanup_inactive, :timer.minutes(5))

    {:ok, %State{cleanup_timer: cleanup_timer}}
  end

  @impl true
  def handle_call({:check_rate, user_id, cost}, _from, state) do
    user_bucket = Map.get(state.users, user_id, new_bucket())

    cond do
      user_bucket.tokens >= cost ->
        # Allow request, consume tokens
        new_bucket = %{user_bucket |
          tokens: user_bucket.tokens - cost,
          last_check: now()
        }
        new_state = put_in(state.users[user_id], new_bucket)
        {:reply, {:ok, new_bucket.tokens}, new_state}

      true ->
        # Rate limited
        Logger.warn("Rate limit exceeded for user #{user_id}")
        :telemetry.execute([:converter, :rate_limit, :exceeded], %{count: 1}, %{user_id: user_id})
        {:reply, {:error, :rate_limited}, state}
    end
  end

  @impl true
  def handle_call({:get_status, user_id}, _from, state) do
    bucket = Map.get(state.users, user_id, new_bucket())
    {:reply, bucket, state}
  end

  @impl true
  def handle_info(:refill_tokens, state) do
    # Refill tokens for all users
    refill_amount = div(@refill_rate * @refill_interval, 1000)

    new_users = state.users
    |> Enum.map(fn {user_id, bucket} ->
      new_tokens = min(bucket.tokens + refill_amount, @max_tokens)
      {user_id, %{bucket | tokens: new_tokens}}
    end)
    |> Map.new()

    # Schedule next refill
    Process.send_after(self(), :refill_tokens, @refill_interval)

    {:noreply, %{state | users: new_users}}
  end

  @impl true
  def handle_info(:cleanup_inactive, state) do
    # Remove users inactive for > 30 minutes
    cutoff = now() - :timer.minutes(30)

    new_users = state.users
    |> Enum.filter(fn {_user_id, bucket} ->
      bucket.last_check > cutoff
    end)
    |> Map.new()

    removed_count = map_size(state.users) - map_size(new_users)
    if removed_count > 0 do
      Logger.info("Cleaned up #{removed_count} inactive rate limit buckets")
    end

    # Schedule next cleanup
    cleanup_timer = Process.send_after(self(), :cleanup_inactive, :timer.minutes(5))

    {:noreply, %{state | users: new_users, cleanup_timer: cleanup_timer}}
  end

  ## Private Functions

  defp new_bucket do
    %{
      tokens: @max_tokens,
      last_check: now()
    }
  end

  defp now, do: System.monotonic_time(:millisecond)
end
