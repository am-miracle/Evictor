#!/bin/sh
set -eu

environment="${1:-dev}"
case "$environment" in
  dev|staging|production) ;;
  *) echo "usage: $0 {dev|staging|production}" >&2; exit 2 ;;
esac

directory="$(pwd)/secrets/$environment"
umask 077
mkdir -p "$directory"

[ -s "$directory/postgres_password" ] || openssl rand -hex 24 > "$directory/postgres_password"
if [ ! -s "$directory/database_url" ]; then
  password="$(tr -d '\n' < "$directory/postgres_password")"
  printf 'postgres://evictor:%s@postgres:5432/evictor?sslmode=disable\n' "$password" > "$directory/database_url"
fi
[ -s "$directory/encryption_key" ] || openssl rand -hex 32 > "$directory/encryption_key"
chmod 600 "$directory"/*
printf 'Secret files ready in %s\n' "$directory"
