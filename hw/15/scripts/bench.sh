#!/usr/bin/env bash
# Простой замер: сколько раз сервис реально дёрнул upstream
# при 200 параллельных одинаковых запросах.
#
# До кэша:           upstream calls ≈ 200, p99 ~ 300ms.
# После TTL-кэша:    upstream calls ≈ 1 на TTL,  p99 < 10ms (попадание).
# С singleflight:    параллельные промахи схлопываются в 1 вызов.
#
# Требует: curl, GNU parallel (или xargs -P).

set -euo pipefail

API="${API:-http://localhost:8080}"
UP="${UP:-http://localhost:9090}"
N="${N:-200}"
Q="${Q:-golang}"

echo "== reset upstream =="
curl -fsS -X POST "$UP/admin/reset"

echo "== fire $N parallel /search?q=$Q =="
start=$(date +%s%N)
seq "$N" | xargs -n1 -P50 -I{} curl -fsS -o /dev/null -w "%{time_total}\n" \
  "$API/search?q=$Q" | sort -n > /tmp/cachesvc-times.txt
end=$(date +%s%N)

echo "== latency =="
total=$(wc -l < /tmp/cachesvc-times.txt | tr -d ' ')
p50_line=$(( total / 2 ))
p99_line=$(( total * 99 / 100 ))
echo "p50: $(sed -n "${p50_line}p" /tmp/cachesvc-times.txt) s"
echo "p99: $(sed -n "${p99_line}p" /tmp/cachesvc-times.txt) s"
printf "wall: %d ms\n" $(( (end - start) / 1000000 ))

echo "== upstream stats =="
curl -fsS "$UP/admin/stats"
