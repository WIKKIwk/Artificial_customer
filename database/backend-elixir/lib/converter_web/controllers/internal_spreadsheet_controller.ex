defmodule ConverterWeb.InternalSpreadsheetController do
  use ConverterWeb, :controller

  alias Converter.CRDT.Spreadsheet, as: SpreadsheetCRDT
  require Logger

  @max_batch_size 1000

  def batch_edit(conn, %{"spreadsheet_id" => spreadsheet_id, "edits" => edits})
      when is_binary(spreadsheet_id) and is_list(edits) do
    with :ok <- authorize(conn),
         :ok <- ensure_crdt_process(spreadsheet_id),
         {:ok, applied} <- apply_edits(spreadsheet_id, edits) do
      json(conn, %{ok: true, applied: applied})
    else
      {:error, :unauthorized} ->
        conn |> put_status(:unauthorized) |> json(%{error: "unauthorized"})

      {:error, :bad_request, message} ->
        conn |> put_status(:bad_request) |> json(%{error: message})

      error ->
        Logger.error("internal batch_edit failed: #{inspect(error)}")
        conn |> put_status(:internal_server_error) |> json(%{error: "internal_error"})
    end
  end

  def batch_edit(conn, _params) do
    case authorize(conn) do
      :ok -> conn |> put_status(:bad_request) |> json(%{error: "invalid_payload"})
      {:error, :unauthorized} -> conn |> put_status(:unauthorized) |> json(%{error: "unauthorized"})
    end
  end

  defp authorize(conn) do
    expected = System.get_env("INTERNAL_API_SECRET")
    provided = conn |> get_req_header("x-internal-secret") |> List.first()

    if is_binary(expected) and expected != "" and provided == expected do
      :ok
    else
      {:error, :unauthorized}
    end
  end

  defp ensure_crdt_process(spreadsheet_id) do
    case Registry.lookup(Converter.CRDT.Registry, spreadsheet_id) do
      [] ->
        case DynamicSupervisor.start_child(
               Converter.CRDT.SpreadsheetSupervisor,
               {SpreadsheetCRDT, spreadsheet_id}
             ) do
          {:ok, _pid} -> :ok
          {:error, {:already_started, _pid}} -> :ok
          other -> other
        end

      [{_pid, _}] ->
        :ok
    end
  end

  defp apply_edits(spreadsheet_id, edits) do
    edits = Enum.take(edits, @max_batch_size)

    Enum.reduce_while(Enum.with_index(edits), {:ok, 0}, fn {edit, idx}, {:ok, applied} ->
      case normalize_edit(edit) do
        {:ok, row, col, value} ->
          # Use user_id=0 for "system" updates (not tied to any connected socket).
          SpreadsheetCRDT.update_cell(spreadsheet_id, row, col, value, 0)
          {:cont, {:ok, applied + 1}}

        {:error, message} ->
          {:halt, {:error, :bad_request, "edit #{idx}: #{message}"}}
      end
    end)
  end

  defp normalize_edit(%{"row" => row, "col" => col} = edit) when is_integer(row) and is_integer(col) do
    cond do
      row < 0 or col < 0 ->
        {:error, "row/col must be >= 0"}

      true ->
        value =
          case Map.get(edit, "value") do
            nil -> ""
            v -> to_string(v)
          end

        {:ok, row, col, value}
    end
  end

  defp normalize_edit(_), do: {:error, "invalid edit shape"}
end

