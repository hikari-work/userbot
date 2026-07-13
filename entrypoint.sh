#!/bin/sh
while true; do
  if [ "$SKIP_BUILD" != "1" ]; then
    echo "========== [BUILDING USERBOT] =========="
    CGO_ENABLED=0 go build -ldflags="-s -w" -o /app/userbot main.go
    BUILD_STATUS=$?
  else
    BUILD_STATUS=0
  fi

  if [ $BUILD_STATUS -eq 0 ]; then
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
