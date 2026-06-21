#!/bin/sh
# Ensure DB directory and file are writable regardless of host user mapping
chmod 777 /app/data 2>/dev/null || true
[ -f /app/data/fund.db ] && chmod 666 /app/data/fund.db 2>/dev/null || true
exec bun main.ts
