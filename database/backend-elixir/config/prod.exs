import Config

# For production, don't forget to configure the url host
# to something meaningful, Phoenix uses this information
# when generating URLs.
config :converter, ConverterWeb.Endpoint,
  url: [host: "localhost", port: 4000],
  http: [
    ip: {0, 0, 0, 0},
    port: String.to_integer(System.get_env("PORT") || "4000"),
    transport_options: [socket_opts: [:inet]]
  ],
  check_origin: false, # Allow all origins for this demo
  server: true

# Do not print debug messages in production
config :logger, level: :info

# Runtime production configuration, including reading
# of environment variables, is done on config/runtime.exs.
