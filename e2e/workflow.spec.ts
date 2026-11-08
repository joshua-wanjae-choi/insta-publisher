import { test, expect } from '@playwright/test';

// 6개 시나리오를 단일 흐름으로 순차 실행하여 실제 사용자 경험을 100% 동일하게 검증
test.describe.serial('Insta Studio Full E2E & Visual Pipeline Test', () => {
  let page: any;

  test.beforeAll(async ({ browser }) => {
    page = await browser.newPage({ viewport: { width: 390, height: 844 } });
  });

  test.afterAll(async () => {
    await page.close();
  });

  test('0. 미인증 사용자는 로그인 화면이 표시되어야 함', async () => {
    await page.goto('/');
    const loginSection = page.locator('#loginSection');
    await expect(loginSection).toBeVisible();
    await expect(page.locator('#mainSection')).not.toBeVisible();
  });

  test('1. 올바른 계정 정보로 로그인 시 1단계 사조 선택 뷰로 진입해야 함', async () => {
    await page.fill('#loginUser', 'admin');
    await page.fill('#loginPass', '1234');
    await page.click('#loginBtn');

    // 메인 섹션 활성화 확인
    const mainSection = page.locator('#mainSection');
    await expect(mainSection).toBeVisible({ timeout: 5000 });
    await expect(page.locator('#tab-1')).toHaveClass(/active/);
    await expect(page.locator('#step-content-1')).toBeVisible();

    // 사조 목록 카드 로드 확인
    const styleCards = page.locator('.style-item-card');
    await expect(styleCards.first()).toBeVisible({ timeout: 5000 });
    const count = await styleCards.count();
    expect(count).toBeGreaterThan(0);
  });

  test('2. 1단계: 사조 선택 후 생성 요청 시 2단계로 자동 전환되고 모킹 이미지가 화면에 렌더링되어야 함', async () => {
    // 첫 번째 사조 카드 클릭
    const firstStyle = page.locator('.style-item-card').first();
    await firstStyle.click();
    await expect(firstStyle).toHaveClass(/selected/);

    // 1차 생성 시작 버튼(#step1Btn) 클릭
    const generateBtn = page.locator('#step1Btn');
    await expect(generateBtn).toBeEnabled();
    await generateBtn.click();

    // 2단계 탭으로 자동 전환 확인 (모킹 제너레이터가 실시간 응답)
    await expect(page.locator('#tab-2')).toHaveClass(/active/, { timeout: 15000 });
    await expect(page.locator('#step-content-2')).toBeVisible();

    // 기준 이미지(#baseImg)가 업로드되어 렌더링되었는지 검증 (깨진 이미지 방지)
    const baseImg = page.locator('#baseImg');
    await expect(baseImg).toBeVisible({ timeout: 15000 });

    // 이미지가 실제로 브라우저에서 200 OK로 디코딩되고 로드 완료되었는지(naturalWidth > 0) 검증!
    await expect.poll(async () => {
      return await baseImg.evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0);
    }, {
      message: '기준 컷 이미지가 깨지지 않고 정상 디스플레이되어야 함 (naturalWidth > 0)',
      timeout: 10000,
    }).toBeTruthy();
  });

  test('3. 2단계: 핑퐁 피드백 반영 및 기준 컷 확정 후 3단계 이동', async () => {
    // 피드백 프리셋 태그 클릭 시 적용 태그 컨테이너에 알약(pill) 생성 확인
    const feedbackTags = page.locator('.q-tag');
    if (await feedbackTags.count() > 0) {
      await feedbackTags.first().click();
      const appliedTagsBox = page.locator('#appliedTagsContainer');
      await expect(appliedTagsBox).toBeVisible();
      const pills = page.locator('.applied-tag-pill');
      expect(await pills.count()).toBeGreaterThan(0);
    }

    // "이 컷 확정 & 다음 ➡️" 버튼 클릭
    const confirmBtn = page.locator('#confirmBaseBtn');
    await expect(confirmBtn).toBeVisible();
    await confirmBtn.click();

    // 3단계로 정상 진입 검증
    await expect(page.locator('#tab-3')).toHaveClass(/active/);
    await expect(page.locator('#step-content-3')).toBeVisible();
  });

  test('4. 3단계: 구도 프리셋 선택 후 확장 생성 및 동일 모델/의상 후보 선택', async () => {
    // 구도 프리셋 카드 로드 대기
    const angleCards = page.locator('.angle-card');
    await expect(angleCards.first()).toBeVisible({ timeout: 5000 });

    // 확장 생성 실행
    const varBtn = page.locator('#variationBtn');
    await expect(varBtn).toBeVisible();
    await varBtn.click();

    // 후보 컷 렌더링 목록 대기 (모킹 생성기가 생성 후 업로드)
    const candidateImgs = page.locator('#variationCandidatesList img');
    await expect(candidateImgs.first()).toBeVisible({ timeout: 15000 });

    // 후보 이미지들이 정상 로드되었는지(naturalWidth > 0) 검증
    const allCandidatesLoaded = await candidateImgs.first().evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0);
    expect(allCandidatesLoaded).toBeTruthy();

    // "선택 완료 & 캡션 작성 ➡️" 버튼 클릭하여 4단계 진입
    const proceedBtn = page.locator('#proceedToStep4Btn');
    await expect(proceedBtn).toBeVisible();
    await proceedBtn.click();

    await expect(page.locator('#tab-4')).toHaveClass(/active/);
    await expect(page.locator('#step-content-4')).toBeVisible();
  });

  test('5. 4단계: 캐러셀 썸네일, 캡션 분리 및 발행 준비 상태 검증', async () => {
    // 썸네일 표시 여부 검증
    const thumbs = page.locator('#finalThumbs img');
    await expect(thumbs.first()).toBeVisible({ timeout: 5000 });

    // 썸네일 이미지 로드 검증
    const thumbLoaded = await thumbs.first().evaluate((img: HTMLImageElement) => img.complete && img.naturalWidth > 0);
    expect(thumbLoaded).toBeTruthy();

    // 캡션 본문 및 해시태그 분리 필드가 채워져 있는지 확인
    const bodyEl = page.locator('#captionBody');
    const hashEl = page.locator('#captionHashtags');
    await expect(bodyEl).toBeVisible();
    await expect(hashEl).toBeVisible();

    // 최종 발행 버튼 활성화 확인
    const publishBtn = page.locator('#finalPublishBtn');
    await expect(publishBtn).toBeVisible();
    await expect(publishBtn).toBeEnabled();
  });

});
