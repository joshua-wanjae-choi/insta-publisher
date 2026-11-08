import { state } from './state';
import { getImageUrl, apiFetchDrafts, apiDeleteDraft, apiSendGenerateJob } from './api';
import { showToast } from './toast';
import { renderVariationCandidates } from './step3';
import { switchStep } from './nav';

export function updateBasePreview() {
  if (!state.baseDraft) return;
  const img = document.getElementById('baseImg') as HTMLImageElement | null;
  const nameSpan = document.getElementById('baseStyleName');
  const src = getImageUrl(state.baseDraft);

  if (img && src) {
    img.src = src + '?t=' + Date.now();
  }
  if (nameSpan) {
    nameSpan.innerText = (state.selectedStyle ? state.selectedStyle.name : '') || state.baseDraft.style_id || 'Vintage Fashion';
  }
}

export function renderAppliedFeedbackTags() {
  const box = document.getElementById('appliedTagsContainer');
  const list = document.getElementById('appliedTagsList');
  if (!box || !list) return;

  if (state.activeFeedbackTags.length === 0) {
    box.style.display = 'none';
    list.innerHTML = '';
    return;
  }

  box.style.display = 'block';
  list.innerHTML = '';
  state.activeFeedbackTags.forEach((text, idx) => {
    const el = document.createElement('span');
    el.className = 'applied-tag-pill';
    el.innerHTML = `${text} <span class="remove-x" style="cursor: pointer; margin-left: 4px;">&times;</span>`;
    el.querySelector('.remove-x')?.addEventListener('click', (e) => {
      e.stopPropagation();
      state.activeFeedbackTags.splice(idx, 1);
      renderAppliedFeedbackTags();
    });
    list.appendChild(el);
  });
}

export function addFeedbackTag(text: string) {
  if (!text) return;
  text = text.trim();
  if (!state.activeFeedbackTags.includes(text)) {
    state.activeFeedbackTags.push(text);
    renderAppliedFeedbackTags();
  }
}

export function clearAllFeedbackTags() {
  state.activeFeedbackTags = [];
  renderAppliedFeedbackTags();
}

export async function requestRefinement() {
  const inputEl = document.getElementById('feedbackInput') as HTMLInputElement | null;
  const custom = inputEl ? inputEl.value.trim() : '';

  let allFeedbacks = [...state.activeFeedbackTags];
  if (custom) allFeedbacks.push(custom);

  if (allFeedbacks.length === 0) {
    return showToast('보완하고 싶은 점을 프리셋 태그에서 선택하거나 직접 입력하세요', true);
  }

  const feedbackStr = allFeedbacks.join(', ');
  const btn = document.getElementById('refineBtn') as HTMLButtonElement | null;
  const loading = document.getElementById('loadingBox2');

  const prevCount = state.drafts.length;
  const initialFirstId = state.drafts.length > 0 ? state.drafts[0].id : null;

  try {
    if (btn) btn.disabled = true;
    if (loading) loading.style.display = 'block';

    const customInputEl = document.getElementById('customInput') as HTMLInputElement | null;
    await apiSendGenerateJob({
      type: 'refine',
      style_id: state.selectedStyle ? state.selectedStyle.id : 'custom',
      style_name: state.selectedStyle ? state.selectedStyle.name : 'Vintage Fashion',
      custom_prompt: customInputEl ? customInputEl.value : '',
      base_image_url: state.baseDraft ? state.baseDraft.preview_url : '',
      feedback: feedbackStr,
      candidate_count: 1
    });

    showToast('맥미니로 핑퐁 보완 재생성 요청 완료!');

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
          state.setBaseDraft(list[0]);
          updateBasePreview();
          renderGallery();
          showToast('✨ 보완된 새 컷이 반영되었습니다!');
        }
      } catch {}
    }, 3000);

  } catch (err: any) {
    if (btn) btn.disabled = false;
    if (loading) loading.style.display = 'none';
    showToast(err.message, true);
  }
}

export function confirmBaseAndNext() {
  if (!state.baseDraft) return showToast('기준 컷이 없습니다', true);
  state.carouselDrafts = [state.baseDraft];
  renderVariationCandidates();
  switchStep(3);
}

export function renderGallery() {
  const grid = document.getElementById('galleryGrid');
  const countEl = document.getElementById('galleryCount');
  if (!grid) return;
  if (countEl) countEl.innerText = state.drafts.length.toString();

  grid.innerHTML = '';
  if (state.drafts.length === 0) {
    grid.innerHTML = '<div style="grid-column: span 3; text-align: center; color: var(--text-muted); font-size: 11px; padding: 14px;">보관된 사진 없음</div>';
    return;
  }

  state.drafts.forEach(d => {
    const card = document.createElement('div');
    const isBase = state.baseDraft && state.baseDraft.id === d.id;
    card.className = 'gallery-card' + (isBase ? ' is-base' : '');

    const previewSrc = getImageUrl(d);
    card.innerHTML = `
      <img src="${previewSrc}?t=${Date.now()}" alt="화보" />
      <div class="gallery-actions">
        <button class="gallery-btn gallery-btn-use">
          ${isBase ? '✓ 현재 기준' : '⚡ 핑퐁 기준으로'}
        </button>
        <button class="gallery-btn gallery-btn-del">
          🗑️
        </button>
      </div>
    `;

    card.querySelector('.gallery-btn-use')?.addEventListener('click', () => {
      state.setBaseDraft(d);
      updateBasePreview();
      renderGallery();
      renderVariationCandidates();
      showToast('선택한 이미지가 핑퐁 기준 컷으로 지정되었습니다!');
    });

    card.querySelector('.gallery-btn-del')?.addEventListener('click', async () => {
      if (!confirm('이 이미지를 완전히 삭제하시겠습니까?')) return;
      try {
        await apiDeleteDraft(d.id);
        const list = await apiFetchDrafts();
        state.setDrafts(list);
        updateBasePreview();
        renderGallery();
        renderVariationCandidates();
        showToast('삭제 완료!');
      } catch (err: any) {
        showToast(err.message, true);
      }
    });

    grid.appendChild(card);
  });
}
