import Config

# Configures the endpoint
config :converter, ConverterWeb.Endpoint,
  url: [host: "localhost"],
  adapter: Phoenix.Endpoint.Cowboy2Adapter,
  render_errors: [
    formats: [json: ConverterWeb.ErrorJSON],
    layout: false
  ],
  pubsub_server: Converter.PubSub,
  live_view: [signing_salt: "ultra-secret-salt"],
  http: [port: 4000],
  server: true

# Configures Elixir's Logger
config :logger, :console,
  format: "$time $metadata[$level] $message\n",
  metadata: [:request_id]

# Use Jason for JSON parsing
config :phoenix, :json_library, Jason

# Configure Redis for distributed state
config :converter, :redis,
  host: System.get_env("REDIS_HOST", "localhost"),
  port: String.to_integer(System.get_env("REDIS_PORT", "6379"))

# Configure database
config :converter, Converter.Repo,
  username: System.get_env("DB_USER", "user"),
  password: System.get_env("DB_PASSWORD", "password"),
  hostname: System.get_env("DB_HOST", "localhost"),
  database: System.get_env("DB_NAME", "converter_db"),
  port: String.to_integer(System.get_env("DB_PORT", "5432")),
  pool_size: 10

# Configure cluster for horizontal scaling
config :libcluster,
  topologies: [
    converter: [
      strategy: Cluster.Strategy.Gossip,
      config: [
        port: 45892,
        if_addr: "0.0.0.0",
        multicast_addr: "230.1.1.251",
        multicast_ttl: 1
      ]
    ]
  ]

import_config "#{config_env()}.exs"
