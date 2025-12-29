defmodule ConverterWeb.MetricsController do
  use ConverterWeb, :controller

  def index(conn, _params) do
    # For now, expose a minimal heartbeat for monitoring.
    json(conn, %{
      uptime_ms: System.monotonic_time(:millisecond),
      node: node(),
      realtime: "phoenix"
    })
  end
end
