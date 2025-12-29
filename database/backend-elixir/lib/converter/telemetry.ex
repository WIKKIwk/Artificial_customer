defmodule Converter.Telemetry do
  use Supervisor
  import Telemetry.Metrics
  alias ConverterWeb.Presence

  def start_link(arg) do
    Supervisor.start_link(__MODULE__, arg, name: __MODULE__)
  end

  @impl true
  def init(_arg) do
    children = [
      {:telemetry_poller, measurements: periodic_measurements(), period: 10_000}
    ]

    Supervisor.init(children, strategy: :one_for_one)
  end

  def metrics do
    [
      # Phoenix Metrics
      summary("phoenix.endpoint.start.system_time",
        unit: {:native, :millisecond}
      ),
      summary("phoenix.endpoint.stop.duration",
        unit: {:native, :millisecond}
      ),
      summary("phoenix.router_dispatch.stop.duration",
        tags: [:route],
        unit: {:native, :millisecond}
      ),

      # Channel Metrics
      summary("phoenix.channel_join.stop.duration",
        unit: {:native, :millisecond}
      ),
      summary("phoenix.channel_handled_in.stop.duration",
        tags: [:event],
        unit: {:native, :millisecond}
      ),

      # VM Metrics
      summary("vm.memory.total", unit: {:byte, :kilobyte}),
      summary("vm.total_run_queue_lengths.total"),
      summary("vm.total_run_queue_lengths.cpu"),
      summary("vm.total_run_queue_lengths.io"),

      # Custom Metrics for Real-time Collaboration
      counter("converter.spreadsheet.edits.count"),
      summary("converter.spreadsheet.edits.latency",
        unit: {:native, :millisecond}
      ),
      last_value("converter.spreadsheet.active_users"),
      counter("converter.spreadsheet.conflicts.resolved")
    ]
  end

  defp periodic_measurements do
    [
      {__MODULE__, :measure_active_users, []}
    ]
  end

  def measure_active_users do
    topics =
      Registry.select(Converter.CRDT.Registry, [{{:"$1", :_, :_}, [], [:"$1"]}])
      |> Enum.map(&"spreadsheet:" <> &1)
      |> Enum.uniq()

    active =
      topics
      |> Enum.reduce(0, fn topic, acc ->
        acc + (Presence.list(topic) |> map_size())
      end)

    :telemetry.execute(
      [:converter, :spreadsheet, :active_users],
      %{count: active},
      %{topics: topics}
    )
  rescue
    _ ->
      :telemetry.execute(
        [:converter, :spreadsheet, :active_users],
        %{count: 0},
        %{}
      )
  end
end
