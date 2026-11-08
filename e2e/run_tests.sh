#!/usr/bin/env bash
set -e

DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ROOT_DIR="$(cd "$DIR/.." && pwd)"

echo "🚀 [E2E Test] Starting Mock Test Server and Generator..."

PORT=8089
UPLOAD_TOKEN="e2e_test_token_999"
BASE_URL="http://127.0.0.1:$PORT"

# 1. 테스트용 임시 디렉터리 준비
TEST_MEDIA_DIR="$DIR/.test_media"
rm -rf "$TEST_MEDIA_DIR"
mkdir -p "$TEST_MEDIA_DIR"

# 2. insta-publisher 테스트 모드로 백그라운드 기동
cd "$DIR"
PORT=$PORT \
PUBLIC_BASE_URL="$BASE_URL" \
UPLOAD_API_TOKEN="$UPLOAD_TOKEN" \
ADMIN_USER="admin" \
ADMIN_PASS="1234" \
./insta-server > "$DIR/.test_server.log" 2>&1 &
SERVER_PID=$!

cleanup() {
    echo "🧹 [E2E Test] Cleaning up test processes..."
    kill $SERVER_PID 2>/dev/null || true
    kill $GEN_PID 2>/dev/null || true
    rm -rf "$TEST_MEDIA_DIR"
}
trap cleanup EXIT

# 서버 준비 대기
echo "⏳ Waiting for publisher to start on port $PORT..."
for i in {1..20}; do
    if curl -s "http://127.0.0.1:$PORT/" >/dev/null 2>&1; then
        echo "✅ Publisher server is ready!"
        break
    fi
    sleep 0.5
done

# 3. insta-generator (MOCK 모드) 백그라운드 기동
cd "$ROOT_DIR/insta-generator"
MOCK_IMAGE_GEN=1 \
PUBLISHER_BASE_URL="$BASE_URL" \
UPLOAD_API_TOKEN="$UPLOAD_TOKEN" \
./insta-generator > "$DIR/.test_generator.log" 2>&1 &
GEN_PID=$!

echo "⏳ Waiting for mock generator to establish SSE connection..."
sleep 2

# 4. Playwright E2E 테스트 실행 (Docker 컨테이너 활용)
echo "🎭 Running Playwright E2E Test Suite..."
cd "$DIR"
docker run --rm \
  --network host \
  -v "$DIR":/app \
  -w /app \
  mcr.microsoft.com/playwright:v1.49.0-jammy \
  npx playwright test --config=playwright.config.ts

echo "🎉 All E2E Tests Passed Successfully!"
