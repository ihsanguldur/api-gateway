#!/usr/bin/env bash
# Brings up the gateway + mock backends via docker compose and runs a small
# scenario suite against the real containers: routing, load balancing,
# health-driven failover, rate limiting, and auth.
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

API_KEY="dev-key-1"
GATEWAY="http://localhost:8080"
FAILED=0

cleanup() {
	echo
	echo "== tearing down =="
	docker compose down >/dev/null 2>&1
}
trap cleanup EXIT

check() {
	local desc="$1" ok="$2" detail="${3:-}"
	if [ "$ok" = "1" ]; then
		echo "  [PASS] $desc"
	else
		echo "  [FAIL] $desc${detail:+ ($detail)}"
		FAILED=1
	fi
}

wait_for() {
	local desc="$1" url="$2" tries="${3:-60}"
	for _ in $(seq 1 "$tries"); do
		if curl -s -o /dev/null -m 2 "$url"; then
			return 0
		fi
		sleep 0.5
	done
	echo "timed out waiting for $desc ($url)" >&2
	return 1
}

echo "== building and starting containers =="
docker compose up -d --build

wait_for "gateway" "$GATEWAY/internal/backends" || exit 1

echo "== waiting for all 3 backends to register and become healthy =="
for _ in $(seq 1 60); do
	COUNT=$(curl -s -m 2 "$GATEWAY/internal/backends" | python3 -c 'import sys,json; b=json.load(sys.stdin); print(sum(1 for x in b if x["Healthy"]))' 2>/dev/null || echo 0)
	[ "$COUNT" = "3" ] && break
	sleep 1
done
check "all 3 backends registered and healthy" "$([ "$COUNT" = "3" ] && echo 1 || echo 0)" "saw $COUNT/3"

echo
echo "== A) routing =="
BODY=$(curl -s -m 5 -H "X-API-Key: $API_KEY" "$GATEWAY/api/users/1")
BACKEND=$(echo "$BODY" | python3 -c 'import sys,json; print(json.load(sys.stdin)["backend"])' 2>/dev/null)
check "/api/users routes to a user-service backend" "$( [[ "$BACKEND" == user-* ]] && echo 1 || echo 0 )" "got backend=$BACKEND"

BODY=$(curl -s -m 5 -H "X-API-Key: $API_KEY" "$GATEWAY/api/orders/1")
BACKEND=$(echo "$BODY" | python3 -c 'import sys,json; print(json.load(sys.stdin)["backend"])' 2>/dev/null)
check "/api/orders routes to the order-service backend" "$( [[ "$BACKEND" == order-a ]] && echo 1 || echo 0 )" "got backend=$BACKEND"

echo
echo "== B) auth (checked early, before the rate-limit test below drains the global per-IP bucket) =="
CODE=$(curl -s -m 5 -o /dev/null -w '%{http_code}' "$GATEWAY/api/users/1")
check "missing API key -> 401" "$( [ "$CODE" = "401" ] && echo 1 || echo 0 )" "got $CODE"

CODE=$(curl -s -m 5 -o /dev/null -w '%{http_code}' -H "X-API-Key: wrong-key" "$GATEWAY/api/users/1")
check "wrong API key -> 401" "$( [ "$CODE" = "401" ] && echo 1 || echo 0 )" "got $CODE"

CODE=$(curl -s -m 5 -o /dev/null -w '%{http_code}' -H "X-API-Key: $API_KEY" "$GATEWAY/api/users/1")
check "correct API key -> 200" "$( [ "$CODE" = "200" ] && echo 1 || echo 0 )" "got $CODE"

echo
echo "== C) load balancing across user-a and user-b =="
SEEN=$(for _ in $(seq 1 10); do
	curl -s -m 5 -H "X-API-Key: $API_KEY" "$GATEWAY/api/users/1" | python3 -c 'import sys,json; print(json.load(sys.stdin)["backend"])' 2>/dev/null
done | sort -u | tr '\n' ' ')
check "both user-a and user-b served traffic" "$( [[ "$SEEN" == *user-a* && "$SEEN" == *user-b* ]] && echo 1 || echo 0 )" "saw: $SEEN"

echo
echo "== D) health-driven failover =="
curl -s -m 5 -X POST "http://localhost:9001/toggle" >/dev/null
echo "  toggled user-a unhealthy, waiting for the health checker to notice..."
sleep 4
ONLY_B=1
for _ in $(seq 1 8); do
	BACKEND=$(curl -s -m 5 -H "X-API-Key: $API_KEY" "$GATEWAY/api/users/1" | python3 -c 'import sys,json; print(json.load(sys.stdin)["backend"])' 2>/dev/null)
	[ "$BACKEND" = "user-a" ] && ONLY_B=0
done
check "unhealthy user-a is no longer selected" "$ONLY_B"

curl -s -m 5 -X POST "http://localhost:9001/toggle" >/dev/null
echo "  toggled user-a healthy again, waiting for it to rejoin..."
sleep 4
SAW_A=0
for _ in $(seq 1 12); do
	BACKEND=$(curl -s -m 5 -H "X-API-Key: $API_KEY" "$GATEWAY/api/users/1" | python3 -c 'import sys,json; print(json.load(sys.stdin)["backend"])' 2>/dev/null)
	[ "$BACKEND" = "user-a" ] && SAW_A=1
done
check "user-a rejoins once healthy again" "$SAW_A"

echo
echo "== E) rate limiting (orders route burst=12) =="
CODES=$(for _ in $(seq 1 20); do
	curl -s -m 5 -o /dev/null -w '%{http_code}\n' -H "X-API-Key: $API_KEY" "$GATEWAY/api/orders/1"
done)
GOT_429=$(echo "$CODES" | grep -c 429)
check "burst above the route limit gets 429s" "$( [ "$GOT_429" -gt 0 ] && echo 1 || echo 0 )" "$GOT_429/20 requests were 429"

echo
if [ "$FAILED" = "0" ]; then
	echo "RESULT: ALL PASS"
else
	echo "RESULT: FAILURES ABOVE"
fi
exit "$FAILED"