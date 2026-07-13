#!/bin/sh
while true; do
  echo "========== [BUILDING USERBOT] =========="
  go build -o /app/userbot main.go
  if [ $? -eq 0 ]; then
    echo "========== [STARTING USERBOT] =========="
    /app/userbot
    EXIT_CODE=$?
    echo "Userbot stopped with exit code: $EXIT_CODE"
    if [ $EXIT_CODE -ne 0 ]; then
      echo "Restarting in 5 seconds due to error..."
      sleep 5
    fi
  else
    echo "Build failed! Retrying in 10 seconds..."
    sleep 10
  fi
done
