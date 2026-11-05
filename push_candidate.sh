#!/bin/bash
# 이 PC에서 생성된 이미지와 캡션을 오라클 서버로 업로드하는 스크립트

SERVER_URL="${1:-http://localhost:8080}"
SECRET_KEY="${2:-supersecret123}"
IMAGE_FILE="$3"
CAPTION="$4"

if [ -z "$IMAGE_FILE" ]; then
  echo "사용법: ./push_candidate.sh <서버주소> <시크릿키> <이미지파일경로> [캡션]"
  echo "예시: ./push_candidate.sh https://insta.mydomain.com mykey ./photo.jpg '오늘의 게시물'"
  exit 1
fi

curl -X POST "${SERVER_URL}/api/upload" \
  -H "X-Admin-Secret: ${SECRET_KEY}" \
  -F "image=@${IMAGE_FILE}" \
  -F "caption=${CAPTION}"

echo ""
