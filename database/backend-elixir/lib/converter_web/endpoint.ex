defmodule ConverterWeb.Endpoint do
  use Phoenix.Endpoint, otp_app: :converter

  # WebSocket transport with optimizations
  socket "/socket", ConverterWeb.UserSocket,
    websocket: [
      timeout: 45_000,
      compress: true,
      max_frame_size: 8_000_000,
      # Enable binary frames for better performance
      serializer: [{Phoenix.Socket.V2.JSONSerializer, "~> 2.0.0"}]
    ],
    longpoll: false

  # CORS plug
  plug Corsica,
    origins: [
      "http://localhost:5173",
      "http://localhost:5500",
      "http://127.0.0.1:5500",
      "http://localhost:3000",
      "http://localhost:8000",
      "http://localhost:8001"
    ],
    allow_headers: ["content-type", "authorization"],
    allow_methods: ["GET", "POST", "PUT", "DELETE", "OPTIONS"]

  plug Plug.Static,
    at: "/",
    from: :converter,
    gzip: false,
    only: ~w(assets fonts images favicon.ico robots.txt)

  if code_reloading? do
    plug Phoenix.CodeReloader
  end

  plug Plug.RequestId
  plug Plug.Telemetry, event_prefix: [:phoenix, :endpoint]

  plug Plug.Parsers,
    parsers: [:urlencoded, :multipart, :json],
    pass: ["*/*"],
    json_decoder: Phoenix.json_library()

  plug Plug.MethodOverride
  plug Plug.Head
  plug ConverterWeb.Router
end
