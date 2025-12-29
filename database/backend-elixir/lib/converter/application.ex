defmodule Converter.Application do
  use Application

  @impl true
  def start(_type, _args) do
    redis_host = System.get_env("REDIS_HOST", "localhost")
    redis_port =
      System.get_env("REDIS_PORT", "6379")
      |> String.to_integer()

    children = [
      # PubSub for distributed messaging
      {Phoenix.PubSub, name: Converter.PubSub},

      # Presence for tracking online users
      ConverterWeb.Presence,

      # Redis connection pool
      {Redix, host: redis_host, port: redis_port, name: :redix},

      # Rate limiter for channel events
      Converter.RateLimiter,

      # Cluster supervisor for horizontal scaling
      {Cluster.Supervisor, [Application.get_env(:libcluster, :topologies), [name: Converter.ClusterSupervisor]]},

      # CRDT supervisor for conflict-free state
      Converter.CRDT.Supervisor,

      # Telemetry for performance monitoring
      Converter.Telemetry,

      # Endpoint (HTTP/WebSocket server)
      ConverterWeb.Endpoint
    ]

    opts = [strategy: :one_for_one, name: Converter.Supervisor]
    Supervisor.start_link(children, opts)
  end

  @impl true
  def config_change(changed, _new, removed) do
    ConverterWeb.Endpoint.config_change(changed, removed)
    :ok
  end
end
