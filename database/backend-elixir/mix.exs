defmodule Converter.MixProject do
  use Mix.Project

  def project do
    [
      app: :converter,
      version: "0.1.0",
      elixir: "~> 1.14",
      start_permanent: Mix.env() == :prod,
      deps: deps()
    ]
  end

  def application do
    [
      mod: {Converter.Application, []},
      extra_applications: [:logger, :runtime_tools, :delta_crdt]
    ]
  end

  defp deps do
    [
      {:phoenix, "~> 1.7.10"},
      {:phoenix_pubsub, "~> 2.1"},
      {:phoenix_live_view, "~> 0.20.1"},
      {:plug_cowboy, "~> 2.6"},
      {:corsica, "~> 2.1"},
      {:joken, "~> 2.6"},
      {:jason, "~> 1.4"},
      {:postgrex, "~> 0.17.3"},
      {:ecto_sql, "~> 3.10"},
      {:delta_crdt, "~> 0.6.4"},
      {:redix, "~> 1.2"},
      {:libcluster, "~> 3.3"},
      {:telemetry, "~> 1.2"},
      {:telemetry_metrics, "~> 0.6"},
      {:telemetry_poller, "~> 1.0"}
    ]
  end
end
