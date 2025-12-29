# Build stage
FROM elixir:1.14-alpine AS build

# Install build dependencies
RUN apk add --no-cache build-base git

# Prepare build directory
WORKDIR /app

# Install hex + rebar
RUN mix local.hex --force && \
    mix local.rebar --force

# Set build env
ENV MIX_ENV=prod

# Copy config and mix files
COPY mix.exs ./
COPY config config

# Install deps
RUN mix deps.get --only prod
RUN mix deps.compile

# Copy lib
COPY lib lib

# Compile
RUN mix compile

# Release
RUN mix release

# Run stage
FROM alpine:3.17

RUN apk add --no-cache libstdc++ ncurses-libs openssl

WORKDIR /app

COPY --from=build /app/_build/prod/rel/converter ./

ENV HOME=/app

CMD ["bin/converter", "start"]
