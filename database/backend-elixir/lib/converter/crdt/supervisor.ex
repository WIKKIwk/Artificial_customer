defmodule Converter.CRDT.Supervisor do
  use Supervisor

  def start_link(init_arg) do
    Supervisor.start_link(__MODULE__, init_arg, name: __MODULE__)
  end

  @impl true
  def init(_init_arg) do
    children = [
      # Dynamic supervisor for spreadsheet CRDTs
      {DynamicSupervisor, name: Converter.CRDT.SpreadsheetSupervisor, strategy: :one_for_one},

      # Registry for looking up CRDT processes
      {Registry, keys: :unique, name: Converter.CRDT.Registry}
    ]

    Supervisor.init(children, strategy: :one_for_one)
  end
end
