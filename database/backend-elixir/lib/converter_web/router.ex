defmodule ConverterWeb.Router do
  use ConverterWeb, :router

  pipeline :api do
    plug :accepts, ["json"]
  end

  scope "/api", ConverterWeb do
    pipe_through :api

    get "/health", HealthController, :index
    get "/metrics", MetricsController, :index
    post "/internal/spreadsheets/:spreadsheet_id/batch_edit", InternalSpreadsheetController, :batch_edit
  end
end
