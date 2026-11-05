# Insta-Publisher ☁️

오라클 클라우드(Caddy 뒤)에서 동작하는 초경량 인스타그램 게시물 발행기 및 웹 검수(Pick) 대시보드입니다.

## 주요 기능
- **단일 바이너리(Go)**: 메모리 ~10MB 수준의 초경량 데몬
- **웹 UI 내장**: 스마트폰 브라우저에서 후보 사진들을 보고 픽(Pick) & 캡션 수정
- **Instagram Graph API 연동**: 메타 공식 API 기반 안전한 피드 발행
- **자동 정리**: 인스타그램 업로드 완료 즉시 로컬 이미지 자동 삭제 (디스크 0% 유지)

## 빠른 시작
1. 설정 파일 복사:
   ```bash
   cp .env.example .env
   # .env에 Instagram Account ID, Graph API Token, ADMIN_SECRET_KEY 설정
   ```
2. 빌드 및 실행:
   ```bash
   go build -o insta-publisher cmd/server/main.go
   ./insta-publisher
   ```
3. Caddyfile 연동 예시:
   ```caddy
   insta.yourdomain.com {
       reverse_proxy localhost:8080
   }
   ```
