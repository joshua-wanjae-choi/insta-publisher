# AGENTS.md — 레트로 패션 AI 인스타 (publisher)

**계정 전체의 목표, 취향, 금지 사항, 플레이북은 `../insta-generator/AGENTS.md`에 있다. 작업 전에 반드시 먼저 읽을 것.**

## 이 저장소의 역할
- 오라클 서버에서 실행된다. generator가 올린 하루 포스트(번들)를 보여주고, 운영자가 승인하면 인스타 캐러셀로 발행한다.
- 첫 탭 **"📅 오늘의 포스트"**(`frontend/src/today.ts`, 서버 코드는 `cmd/server/main.go` 하단 "Daily bundles")
  - `GET /api/bundles`: `bundle_id`(`day-YYYY-MM-DD`)로 묶인 초안 목록
  - `POST /api/bundles/approve`: 순서대로 캐러셀을 발행하고 SSE `approved` 이벤트를 보낸다. generator가 이를 받아 조합 얼굴을 뮤즈로 승격한다. 두 번 눌러도 한 번만 발행된다.
  - `POST /api/bundles/regenerate`: 초안을 삭제하고 SSE `regenerate` 이벤트를 보낸다. generator가 재생성한다.
- 1~4단계 탭은 예전 수동 흐름이다. 운영자는 미세 조정을 최소화하길 원하니, 이 흐름을 키우지 말 것.

## 원칙
- 승인 화면은 [승인], [다시], 캡션 수정만 둔다. 프롬프트 조정 UI를 추가하지 말 것.
- `styles.json`에 실존 인물, 영화 제목, 브랜드명을 넣지 말 것.
- 발행은 실제 인스타 계정에 올라간다. 테스트할 때는 인스타 인증 정보 없이 서버를 띄울 것(`.env`가 없는 폴더에서 실행).

## 배포 (중요)
- **태그를 푸시하면 자동 배포된다.** GitHub 웹훅 → 오라클 서버의 `oci-webserver/webhook/deploy-tag.sh` → 해당 태그 체크아웃 → `build.sh insta-publisher` → `up.sh`. 예: `git tag v1.0.8 && git push origin v1.0.8`
- 서버 빌드 명령(`oci-webserver/services.sh`)이 **파일을 하나씩 지정**한다: `cmd/server/main.go sse.go styles.go security.go`. **`cmd/server`에 새 .go 파일을 만들면 배포가 깨진다.** 새 코드는 이 네 파일 중 하나에 넣는다.
- 프론트엔드는 서버에서 빌드하지 않는다. **`npm run build` 결과(`cmd/server/web/static`)를 커밋해야** 반영된다.
- 태그를 올리기 전에 반드시 서버와 같은 명령으로 확인한다:
  `CGO_ENABLED=0 go build -o /tmp/x cmd/server/main.go cmd/server/sse.go cmd/server/styles.go cmd/server/security.go`
- 헤더의 버전 표시(`frontend/index.html`)를 태그와 맞춰 올리면, 배포 후 화면에서 반영 여부를 바로 확인할 수 있다.

## 빌드
```bash
npm install && npm run build      # frontend → cmd/server/web/static (embed)
go vet ./cmd/server && go build -o insta-server ./cmd/server
```
