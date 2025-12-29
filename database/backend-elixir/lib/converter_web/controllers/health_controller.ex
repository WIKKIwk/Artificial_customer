defmodule ConverterWeb.HealthController do
  use ConverterWeb, :controller

  def index(conn, _params) do
    json(conn, %{status: "ok"})
  end
end
