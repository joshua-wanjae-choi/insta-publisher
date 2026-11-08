import { state } from './state';
import { getImageUrl, apiFetchDrafts, apiDeleteDraft, apiSendGenerateJob } from './api';
import { showToast } from './toast';
import { updateBasePreview, renderGallery } from './step2';
import { renderFinalThumbs } from './step4';
import { switchStep } from './nav';

const defaultAngles = [
  { title: '🚶 전신 런웨이/워킹', desc: '착장 전체 비율 & 실루엣', prompt: 'Full-body candid runway walk, head to toe' },
  { title: '🔍 원단/소재 클로즈업', desc: '가죽/캐시미어 원단 질감', prompt: 'Extreme close-up on fabric texture, stitching and accessories' },
  { title: '👤 상반신 캔디드 스냅', desc: '표정 및 넥라인/재킷 상의', prompt: 'Medium bust shot candid expression, natural framing' },
  { title: '💫 무빙 / 모션 블러', desc: '역동적인 90년대 스냅 감성', prompt: 'Atmospheric motion blur walking shot, dynamic editorial mood' },
  { title: '👠 슈즈 & 헴라인 디테일', desc: '신발과 바지 밑단 핏', prompt: 'Low-angle close-up on footwear and trouser hemline on street pavement' },
  { title: '🔄 뒤돌아보는 오버숄더', desc: '백 실루엣과 등판 디테일', prompt: 'Back view looking back over the shoulder, highlighting coat back design' },
  { title: '☕ 카페/거리 착석 샷', desc: '자연스럽게 앉아있는 룩', prompt: 'Effortless sitting pose at an outdoor vintage café chair or curb' },
  { title: '📐 로우앵글 파워 샷', desc: '웅장하고 슬림한 하이패션', prompt: 'Dramatic low-angle worm-eye view emphasizing tall power silhouette' }
];

export function renderAngleGrid() {
  const grid = document.getElementById('angleGrid');
  const countBadge = document.getElementById('selectedCountBadge');
  if (!grid) return;
  grid.innerHTML = '';

  const allAngles = [...defaultAngles, ...state.customAngles];
  if (countBadge) {
    countBadge.innerText = `${state.selectedAngles.length}개 선택됨`;
  }

  allAngles.forEach(item => {
    const isSelected = state.selectedAngles.includes(item.prompt);
    const card = document.createElement('div');
    card.className = 'angle-card' + (isSelected ? ' selected' : '');

    card.innerHTML = `
      <div class="angle-title">
        <span>${item.title}</span>
        <span class="angle-check">✓</span>
      </div>
      <div class="angle-desc">${item.desc}</div>
    `;

    card.addEventListener('click', () => {
      if (isSelected) {
        state.selectedAngles = state.selectedAngles.filter(p => p !== item.prompt);
      } else {
        if (state.selectedAngles.length >= 4) {
          showToast('구도는 최대 4개까지만 선택 가능합니다', true);
          return;
        }
        state.selectedAngles.push(item.prompt);
      }
      renderAngleGrid();
    });

    grid.appendChild(card);
  });
}

export function addCustomAngle() {
  const inputEl = document.getElementById('customAngleInput') as HTMLInputElement | null;
  if (!inputEl) return;
  const val = inputEl.value.trim();
  if (!val) return;

  state.customAngles.push({
    title: '✨ ' + val.substring(0, 12),
    desc: val,
    prompt: val
  });
  state.selectedAngles.push(val);
  inputEl.value = '';
  renderAngleGrid();
}

export async function requestVariations() {
  if (state.selectedAngles.length === 0) {
    return showToast('구도를 최소 1개 이상 선택하세요', true);
  }

  const btn = document.getElementById('variationBtn') as HTMLButtonElement | null;
  const loading = document.getElementById('loadingBox3');

  const prevCount = state.drafts.length;
  const initialFirstId = state.drafts.length > 0 ? state.drafts[0].id : null;

  const baseDesc = state.baseDraft ? state.baseDraft.caption.substring(0, 200) : '';
  const initialPromptEl = document.getElementById('customInput') as HTMLInputElement | null;
  const initialPrompt = initialPromptEl ? initialPromptEl.value : '';
  const combinedPrompt = initialPrompt + (baseDesc ? ` (Maintain exact same model, face, outfit and aesthetic from base shot: ${baseDesc})` : '');

  try {
    if (btn) btn.disabled = true;
    if (loading) loading.style.display = 'block';

    await apiSendGenerateJob({
      type: 'variations',
      style_id: state.selectedStyle ? state.selectedStyle.id : 'custom',
      style_name: state.selectedStyle ? state.selectedStyle.name : 'Fashion',
      custom_prompt: combinedPrompt,
      base_image_url: state.baseDraft ? state.baseDraft.preview_url : '',
      feedback: state.activeFeedbackTags.join(', '),
      preset_angles: state.selectedAngles
    });

    showToast('구도 프리셋 화보 확장 생성 시작!');

    let elapsed = 0;
    const timer = setInterval(async () => {
      elapsed += 3;
      if (elapsed > 120) {
        clearInterval(timer);
        if (btn) btn.disabled = false;
        if (loading) loading.style.display = 'none';
        showToast('생성 시간 초과. 다시 시도해주세요.', true);
        return;
      }

      try {
        const list = await apiFetchDrafts();
        const currentFirstId = list.length > 0 ? list[0].id : null;
        if (list.length > prevCount || (list.length > 0 && currentFirstId !== initialFirstId)) {
          clearInterval(timer);
          if (btn) btn.disabled = false;
          if (loading) loading.style.display = 'none';

          state.setDrafts(list);
          renderVariationCandidates();
          renderGallery();
          renderFinalThumbs();
          showToast('🎉 바리에이션 사진 생성이 완료되었습니다!');
        }
      } catch {}
    }, 3000);

  } catch (err: any) {
    if (btn) btn.disabled = false;
    if (loading) loading.style.display = 'none';
    showToast(err.message, true);
  }
}

export function renderVariationCandidates() {
  const container = document.getElementById('variationCandidatesList');
  const badge = document.getElementById('carouselSelectedBadge');
  if (!container) return;
  if (badge) badge.innerText = state.carouselDrafts.length.toString();

  container.innerHTML = '';
  if (state.drafts.length === 0) {
    container.innerHTML = '<div style="grid-column: span 2; text-align: center; color: var(--text-muted); font-size: 12px; padding: 20px;">생성된 사진이 없습니다. 위에서 구도를 선택해 생성하세요!</div>';
    return;
  }

  state.drafts.forEach(d => {
    const orderIdx = state.carouselDrafts.findIndex(cd => cd.id === d.id);
    const inCarousel = orderIdx >= 0;
    const isBase = state.baseDraft && state.baseDraft.id === d.id;

    const card = document.createElement('div');
    card.className = 'candidate-card' + (inCarousel ? ' in-carousel' : '');

    const previewSrc = getImageUrl(d);
    card.innerHTML = `
      <div style="position: relative;">
        <img src="${previewSrc}" alt="후보 이미지" loading="lazy" />
        ${isBase ? '<span style="position: absolute; top: 6px; left: 6px; background: #3b82f6; color: #fff; font-size: 10px; font-weight: 700; padding: 2px 6px; border-radius: 4px;">기준 컷</span>' : ''}
      </div>
      <div class="candidate-info">
        <div class="candidate-meta">${d.caption ? d.caption.substring(0, 30) + '...' : '확장 컷'}</div>
        <div class="candidate-btn-group">
          <button class="candidate-btn btn-toggle-order ${inCarousel ? 'btn-order-active' : 'btn-outline'}">
            ${inCarousel ? `✓ #${orderIdx + 1}번째` : '+ 캐러셀 담기'}
          </button>
          <button class="candidate-btn btn-del btn-outline" style="flex: 0.4; color: #f87171;">
            🗑️
          </button>
        </div>
      </div>
    `;

    card.querySelector('.btn-toggle-order')?.addEventListener('click', () => {
      toggleCarouselSelection(d.id);
    });

    card.querySelector('.btn-del')?.addEventListener('click', async () => {
      if (!confirm('이 이미지를 완전히 삭제하시겠습니까?')) return;
      try {
        await apiDeleteDraft(d.id);
        const list = await apiFetchDrafts();
        state.setDrafts(list);
        updateBasePreview();
        renderGallery();
        renderVariationCandidates();
        renderFinalThumbs();
        showToast('삭제 완료!');
      } catch (err: any) {
        showToast(err.message, true);
      }
    });

    container.appendChild(card);
  });
}

export function toggleCarouselSelection(draftId: string) {
  const existingIdx = state.carouselDrafts.findIndex(cd => cd.id === draftId);
  if (existingIdx >= 0) {
    state.carouselDrafts.splice(existingIdx, 1);
  } else {
    if (state.carouselDrafts.length >= 4) {
      showToast('인스타그램 캐러셀은 최대 4장까지 담을 수 있습니다', true);
      return;
    }
    const target = state.drafts.find(d => d.id === draftId);
    if (target) {
      state.carouselDrafts.push(target);
    }
  }
  renderVariationCandidates();
  renderFinalThumbs();
}

export function proceedToStep4() {
  if (state.carouselDrafts.length === 0) {
    return showToast('캐러셀에 포함할 사진을 최소 1장 이상 선택하세요', true);
  }
  renderFinalThumbs();
  switchStep(4);
}
