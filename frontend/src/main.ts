import { state } from './state';
import { apiFetchStyles, apiFetchDrafts } from './api';
import { showToast } from './toast';
import { renderStyles, filterEra, startInitialGeneration } from './step1';
import { 
  addFeedbackTag, 
  clearAllFeedbackTags, 
  requestRefinement, 
  confirmBaseAndNext, 
  updateBasePreview, 
  renderGallery 
} from './step2';
import { 
  addCustomAngle, 
  requestVariations, 
  proceedToStep4, 
  renderAngleGrid, 
  renderVariationCandidates 
} from './step3';
import { publishPost, resetWorkflow } from './step4';
import { switchStep } from './nav';

let isMFARequired = false;

async function checkSession() {
  try {
    const res = await fetch('/api/styles');
    if (!res.ok) throw new Error();
    showMainView();
    await loadAll();
  } catch {
    showLoginView();
  }
}

function showMainView() {
  const loginSection = document.getElementById('loginSection');
  const mainSection = document.getElementById('mainSection');
  if (loginSection) loginSection.style.display = 'none';
  if (mainSection) mainSection.style.display = 'block';
}

function showLoginView() {
  const loginSection = document.getElementById('loginSection');
  const mainSection = document.getElementById('mainSection');
  if (loginSection) loginSection.style.display = 'block';
  if (mainSection) mainSection.style.display = 'none';
}

async function handleLogin() {
  const u = (document.getElementById('loginUser') as HTMLInputElement | null)?.value.trim() || '';
  const p = (document.getElementById('loginPass') as HTMLInputElement | null)?.value.trim() || '';
  const mfa = (document.getElementById('loginMFA') as HTMLInputElement | null)?.value.trim() || '';
  const btn = document.getElementById('loginBtn') as HTMLButtonElement | null;

  if (btn) {
    btn.disabled = true;
    btn.innerText = '인증 확인 중...';
  }

  const payload: any = { username: u, password: p };
  if (isMFARequired || mfa) {
    payload.mfa_code = mfa;
  }

  try {
    const res = await fetch('/api/login', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload)
    });
    const data = await res.json().catch(() => ({}));

    if (res.status === 202 || data.mfa_required) {
      isMFARequired = true;
      const mfaBox = document.getElementById('mfaInputBox');
      if (mfaBox) mfaBox.style.display = 'block';
      (document.getElementById('loginMFA') as HTMLInputElement | null)?.focus();
      showToast('구글 OTP 6자리 번호를 입력하세요');
      if (btn) btn.innerText = 'OTP 인증 완료하기';
      return;
    }

    if (!res.ok) throw new Error(data.error || '로그인 실패');

    showToast('로그인 성공!');
    showMainView();
    await loadAll();
  } catch (err: any) {
    showToast(err.message, true);
  } finally {
    if (btn) {
      btn.disabled = false;
      if (!isMFARequired) btn.innerText = '로그인하기';
    }
  }
}

async function handleLogout() {
  try {
    await fetch('/api/logout', { method: 'POST' });
  } catch {}
  showLoginView();
  showToast('로그아웃 되었습니다');
}

async function loadAll() {
  try {
    const [stylesData, draftsData] = await Promise.all([
      apiFetchStyles().catch(() => []),
      apiFetchDrafts().catch(() => [])
    ]);

    state.setStyles(stylesData);
    state.setDrafts(draftsData);

    renderStyles();
    updateBasePreview();
    renderGallery();
    renderAngleGrid();
    renderVariationCandidates();
  } catch (err: any) {
    showToast('데이터 로드 실패: ' + err.message, true);
  }
}

function bindEvents() {
  document.getElementById('loginBtn')?.addEventListener('click', handleLogin);
  document.getElementById('logoutBtn')?.addEventListener('click', handleLogout);
  document.getElementById('refreshAllBtn')?.addEventListener('click', loadAll);

  // 탭 네비게이션
  for (let i = 1; i <= 4; i++) {
    document.getElementById(`tab-${i}`)?.addEventListener('click', () => switchStep(i));
  }

  // 1단계 시대 필터 탭
  document.getElementById('eraTabs')?.addEventListener('click', (e) => {
    const target = (e.target as HTMLElement).closest('.era-tab') as HTMLElement | null;
    if (target && target.dataset.era) {
      filterEra(target.dataset.era, target);
    }
  });

  // 1단계 생성 버튼
  document.getElementById('step1Btn')?.addEventListener('click', startInitialGeneration);

  // 2단계 프리셋 태그
  document.getElementById('quickTags')?.addEventListener('click', (e) => {
    const target = (e.target as HTMLElement).closest('.q-tag') as HTMLElement | null;
    if (target && target.dataset.tag) {
      addFeedbackTag(target.dataset.tag);
    }
  });
  document.getElementById('clearTagsBtn')?.addEventListener('click', clearAllFeedbackTags);
  document.getElementById('feedbackInput')?.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Enter') requestRefinement();
  });
  document.getElementById('refineBtn')?.addEventListener('click', requestRefinement);
  document.getElementById('confirmBaseBtn')?.addEventListener('click', confirmBaseAndNext);

  // 3단계 커스텀 구도 및 생성
  document.getElementById('addCustomAngleBtn')?.addEventListener('click', addCustomAngle);
  document.getElementById('customAngleInput')?.addEventListener('keydown', (e: KeyboardEvent) => {
    if (e.key === 'Enter') addCustomAngle();
  });
  document.getElementById('variationBtn')?.addEventListener('click', requestVariations);
  document.getElementById('proceedToStep4Btn')?.addEventListener('click', proceedToStep4);

  // 4단계 발행 및 리셋
  document.getElementById('finalPublishBtn')?.addEventListener('click', publishPost);
  document.getElementById('resetWorkflowBtn')?.addEventListener('click', resetWorkflow);
}

// 초기 진입
window.addEventListener('DOMContentLoaded', () => {
  bindEvents();
  checkSession();
});
