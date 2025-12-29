defmodule ConverterWeb.UserSocket do
  use Phoenix.Socket
  alias ConverterWeb.AuthToken

  channel "spreadsheet:*", ConverterWeb.SpreadsheetChannel

  @impl true
  def connect(%{"token" => token} = params, socket, _connect_info) do
    case AuthToken.verify(token) do
      {:ok, user_id} ->
        {:ok,
          socket
          |> assign(:params, Map.put(params, "user_id", user_id))
          |> assign(:user_id, user_id)}

      _ ->
        :error
    end
  end

  def connect(_params, _socket, _connect_info) do
    :error
  end

  @impl true
  def id(%{assigns: %{user_id: user_id}}), do: "user_socket:#{user_id}"
  def id(_socket), do: nil
end
