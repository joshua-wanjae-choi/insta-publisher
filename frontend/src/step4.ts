import { state } from './state';
import { getImageUrl, apiPublishCarousel } from './api';
import { showToast } from './toast';
import { switchStep } from './nav';

export function renderFinalThumbs() {
  const container = document.getElementById('finalThumbs');
  if (!container) return;
  container.innerHTML = '';

  state.carouselDrafts.forEach((d, idx) => {
    const box = document.createElement('div');
    box.style.cssText = 'position: relative; width: 70px; height: 88px; flex-shrink: 0; border-radius: 8px; overflow: hidden; border: 2px solid #3b82f6;';

    const img = document.createElement('img');
    img.src = getImageUrl(d);
    img.style.cssText = 'width: 100%; height: 100%; object-fit: cover;';

    const badge = document.createElement('span');
    badge.style.cssText = 'position: absolute; top: 2px; left: 2px; background: rgba(0,0,0,0.8); color: #60a5fa; font-size: 10px; font-weight: bold; padding: 1px 5px; border-radius: 4px;';
    badge.innerText = `#${idx + 1}`;

    box.appendChild(img);
    box.appendChild(badge);
    container.appendChild(box);
  });

  const bodyEl = document.getElementById('captionBody') as HTMLTextAreaElement | null;

  if (state.baseDraft && bodyEl && !bodyEl.value) {
    extractCaptionParts(state.baseDraft.caption);
  }
}

export function extractCaptionParts(fullText: string) {
  const bodyEl = document.getElementById('captionBody') as HTMLTextAreaElement | null;
  const hashEl = document.getElementById('captionHashtags') as HTMLInputElement | null;
  if (!fullText) return;

  const lines = fullText.split('\n');
  const hashtags: string[] = [];
  const bodyLines: string[] = [];

  lines.forEach(l => {
    const trimmed = l.trim();
    if (trimmed.startsWith('#')) {
      const tags = trimmed.match(/#[a-z0-9_]+/gi);
      if (tags) hashtags.push(...tags);
    } else {
      bodyLines.push(l);
    }
  });

  if (bodyEl) bodyEl.value = bodyLines.join('\n').trim();
  if (hashEl) hashEl.value = hashtags.join(' ');
}

export async function publishPost() {
  if (state.carouselDrafts.length === 0) {
    return showToast('발행할 사진을 최소 1장 이상 선택하세요', true);
  }

  const bodyEl = document.getElementById('captionBody') as HTMLTextAreaElement | null;
  const hashEl = document.getElementById('captionHashtags') as HTMLInputElement | null;

  const fullCaption = `${bodyEl ? bodyEl.value.trim() : ''}\n\n${hashEl ? hashEl.value.trim() : ''}`.trim();
  const ids = state.carouselDrafts.map(d => d.id);

  const btn = document.getElementById('finalPublishBtn') as HTMLButtonElement | null;
  if (btn) {
    btn.disabled = true;
    btn.innerText = '인스타그램 업로드 & 발행 중...';
  }

  try {
    const res = await apiPublishCarousel(ids, fullCaption);
    showToast(`🎉 인스타그램 캐러셀 발행 성공! (Media ID: ${res.media_id})`);
    setTimeout(() => {
      resetWorkflow();
    }, 2500);
  } catch (err: any) {
    showToast(err.message, true);
  } finally {
    if (btn) {
      btn.disabled = false;
      btn.innerText = '🚀 인스타그램에 바로 올리기 (캐러셀 발행)';
    }
  }
}

export function resetWorkflow() {
  state.carouselDrafts = [];
  state.baseDraft = null;
  state.activeFeedbackTags = [];
  switchStep(1);
}
