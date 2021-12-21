#!/bin/sh

if [ -z "$GOGROK_ARGS" ]; then
  GOGROK_ARGS="serve"
fi

/usr/bin/gogrok $GOGROK_ARGS