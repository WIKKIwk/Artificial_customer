defmodule ConverterWeb.AuthToken do
  @moduledoc """
  Minimal JWT verification shared between socket connect/join.
  Matches the Go backend's HS256 tokens (sub or user_id).
  """
  require Logger

  @spec verify(String.t()) :: {:ok, integer()} | {:error, :invalid_token}
  def verify(token) when is_binary(token) do
    with {:ok, claims} <- verify_claims(token),
         {:ok, user_id} <- user_id_from_claims(claims) do
      {:ok, user_id}
    else
      _ ->
        {:error, :invalid_token}
    end
  rescue
    _ -> {:error, :invalid_token}
  end

  def verify(_), do: {:error, :invalid_token}

  @spec verify_claims(String.t()) :: {:ok, map()} | {:error, :invalid_token}
  def verify_claims(token) when is_binary(token) do
    with {:ok, secret} <- secret(),
         [header, payload, signature] <- String.split(token, "."),
         true <- header != "" and payload != "" and signature != "",
         {:ok, claims} <- decode_payload(payload),
         :ok <- check_expiration(claims),
         true <- valid_signature?(secret, header, payload, signature) do
      {:ok, claims}
    else
      _ -> {:error, :invalid_token}
    end
  rescue
    _ -> {:error, :invalid_token}
  end

  def verify_claims(_), do: {:error, :invalid_token}

  @spec user_id_from_claims(map()) :: {:ok, integer()} | {:error, :invalid_token}
  def user_id_from_claims(claims) when is_map(claims), do: extract_user_id(claims)
  def user_id_from_claims(_), do: {:error, :invalid_token}

  ## Private

  defp check_expiration(%{"exp" => exp_timestamp}) when is_integer(exp_timestamp) do
    now_unix = DateTime.utc_now() |> DateTime.to_unix()

    if exp_timestamp > now_unix do
      :ok
    else
      Logger.warn("Attempted to use expired JWT.")
      {:error, :invalid_token}
    end
  end

  defp check_expiration(_claims) do
    Logger.warn("Received JWT without 'exp' claim.")
    {:error, :invalid_token}
  end

  defp decode_payload(payload_segment) do
    payload_segment
    |> Base.url_decode64!(padding: false)
    |> Jason.decode()
  end

  defp extract_user_id(%{"user_id" => id}), do: normalize_user_id(id)
  defp extract_user_id(%{"sub" => id}), do: normalize_user_id(id)
  defp extract_user_id(_), do: {:error, :invalid_token}

  defp normalize_user_id(id) when is_integer(id), do: {:ok, id}

  defp normalize_user_id(id) when is_binary(id) do
    case Integer.parse(id) do
      {val, _} -> {:ok, val}
      :error -> {:error, :invalid_token}
    end
  end

  defp normalize_user_id(_), do: {:error, :invalid_token}

  defp valid_signature?(secret, header, payload, signature) do
    data = header <> "." <> payload
    expected =
      :crypto.mac(:hmac, :sha256, secret, data)
      |> Base.url_encode64(padding: false)

    secure_compare(expected, signature)
  end

  defp secure_compare(a, b) when byte_size(a) == byte_size(b) do
    Plug.Crypto.secure_compare(a, b)
  rescue
    _ -> false
  end

  defp secure_compare(_, _), do: false

  defp secret do
    case System.get_env("JWT_SECRET") do
      nil ->
        Logger.error("JWT_SECRET is not set for AuthToken verification.")
        {:error, :invalid_token}

      secret when byte_size(secret) < 32 ->
        Logger.error("JWT_SECRET is too short for secure verification.")
        {:error, :invalid_token}

      secret ->
        {:ok, secret}
    end
  end
end
