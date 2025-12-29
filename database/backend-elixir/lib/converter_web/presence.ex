defmodule ConverterWeb.Presence do
  use Phoenix.Presence, otp_app: :converter_web,
                        pubsub_server: Converter.PubSub
end
