import { state } from './state';
import { apiSendGenerateJob, apiFetchDrafts } from './api';
import { showToast } from './toast';
import { updateBasePreview, renderGallery } from './step2';

export function renderStyles() {
  const list = document.getElementById('styleList');
  if (!list) return;
  list.innerHTML = '';

  const filtered = (state.currentEra === 'ALL')
    ? state.styles
    : state.styles.filter(s => {
        if (!s.era) return false;
        if (state.currentEra === 'Classic') return s.era === 'Classic';
        if (state.currentEra === '1900s') return s.era.startsWith('1900') || s.era.startsWith('1910');
        return s.era.includes(state.currentEra.substring(0, 4));
      });

  filtered.forEach(item => {
    const card = document.createElement('div');
    const isSelected = state.selectedStyle && state.selectedStyle.id === item.id;
    card.className = 'style-item-card' + (isSelected ? ' selected' : '');
    card.onclick = () => {
      state.selectedStyle = item;
      renderStyles();
    };

    card.innerHTML = `
      <div class="item-header">
        <span class="item-name">${item.name}</span>
        <span class="item-era-badge">${item.era || 'Vintage'}</span>
      </div>
      <div class="item-desc">${item.desc}</div>
      ${item.keywords ? `<div class="item-keywords">🔑 ${item.keywords}</div>` : ''}
    `;
    list.appendChild(card);
  });

  if (!state.selectedStyle && filtered.length > 0) {
    state.selectedStyle = filtered[0];
  }
}

export function filterEra(era: string, tabEl: HTMLElement | null) {
  state.currentEra = era;
  document.querySelectorAll('.era-tab').forEach(t => t.classList.remove('active'));
  if (tabEl) tabEl.classList.add('active');
  renderStyles();
}

export async function startInitialGeneration() {
  let style = state.selectedStyle;
  if (!style && state.styles.length > 0) {
    style = state.styles[0];
    state.selectedStyle = style;
  }
  if (!style) {
    style = { id: 'vintage-default', era: 'Vintage', name: 'Vintage Fashion', desc: '' };
  }

  const customEl = document.getElementById('customInput') as HTMLInputElement | null;
  const custom = customEl ? customEl.value.trim() : '';

  const btn = document.getElementById('step1Btn') as HTMLButtonElement | null;
  const loading = document.getElementById('loadingBox1');

  const prevCount = state.drafts.length;
  const initialFirstId = state.drafts.length > 0 ? state.drafts[0].id : null;

  try {
    if (btn) btn.disabled = true;
    if (loading) loading.style.display = 'block';

    await apiSendGenerateJob({
      type: 'initial',
      style_id: style.id,
      style_name: style.name,
      custom_prompt: custom,
      candidate_count: 1
    });

    showToast('맥미니로 최초 컷 생성 요청 완료!');

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
          updateBasePreview();
          renderGallery();
          showToast('🎉 최초 컷 생성 완료! 2단계로 이동합니다.');

          // 2단계로 전환 이벤트
          import('./nav').then(m => m.switchStep(2));
        }
      } catch {}
    }, 3000);

  } catch (err: any) {
    if (btn) btn.disabled = false;
    if (loading) loading.style.display = 'none';
    showToast(err.message, true);
  }
}
