#!/usr/bin/env bash

# Config
START="42-1"
END="44-1"

echo "Cleaning up old data..."
make clean

echo "Downloading votes from Parliament sessions $START to $END..."
go run ./cmd/download --start "$START" --end "$END"

echo "Importing votes into database..."
go run ./cmd/import

echo "Bootstrap complete. You can now query the database."

