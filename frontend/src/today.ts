import type { Bundle } from './types';
import { apiFetchBundles, apiApproveBundle, apiRegenerateBundle, getImageUrl } from './api';
import { showToast } from './toast';

// "오늘의 포스트": the generator renders one bundle a day. The operator only approves or asks
// for a redo — no prompt tweaking here on purpose.

function el<K extends keyof HTMLElementTagNameMap>(tag: K, css = '', text = ''): HTMLElementTagNameMap[K] {
  const e = document.createElement(tag);
  if (css) e.style.cssText = css;
  if (text) e.textContent = text;
  return e;
}

function dayLabel(id: string): string {
  const m = id.match(/^day-(\d{4})-(\d{2})-(\d{2})$/);
  return m ? `${m[1]}.${m[2]}.${m[3]}` : id;
}

function renderBundle(b: Bundle): HTMLElement {
  const card = el('div', 'border: 1px solid rgba(255,255,255,0.12); border-radius: 14px; padding: 12px; margin-bottom: 16px;');
  card.appendChild(el('div', 'font-weight: 700; margin-bottom: 8px;', `${dayLabel(b.id)} · ${b.drafts.length}장`));

  // swipeable strip, in carousel order
  const strip = el('div', 'display: flex; gap: 8px; overflow-x: auto; scroll-snap-type: x mandatory; padding-bottom: 6px;');
  b.drafts.forEach((d, i) => {
    const box = el('div', 'position: relative; flex: 0 0 78%; max-width: 340px; scroll-snap-align: start; border-radius: 10px; overflow: hidden; aspect-ratio: 4 / 5; background: #111;');
    const img = el('img', 'width: 100%; height: 100%; object-fit: cover; display: block;');
    img.src = getImageUrl(d);
    img.loading = 'lazy';
    img.alt = `${i + 1}번째 사진`;
    img.addEventListener('click', () => window.open(img.src, '_blank'));
    box.appendChild(img);
    box.appendChild(el('span', 'position: absolute; top: 6px; left: 6px; background: rgba(0,0,0,0.7); color: #fff; font-size: 11px; padding: 1px 6px; border-radius: 6px;', `${i + 1}/${b.drafts.length}`));
    strip.appendChild(box);
  });
  card.appendChild(strip);

  const caption = el('textarea', 'margin-top: 10px; min-height: 84px;');
  caption.value = b.caption;
  card.appendChild(caption);

  const row = el('div', 'display: flex; gap: 8px; margin-top: 8px;');
  const approve = el('button', 'flex: 2;', '승인하고 업로드');
  approve.className = 'btn btn-primary';
  const redo = el('button', 'flex: 1;', '다시 만들기');
  redo.className = 'btn btn-outline';
  row.append(approve, redo);
  card.appendChild(row);

  const setBusy = (busy: boolean, label?: string) => {
    approve.disabled = redo.disabled = busy;
    if (label) approve.textContent = label;
  };

  approve.addEventListener('click', async () => {
    if (!confirm('이 포스트를 인스타그램에 올릴까요?')) return;
    setBusy(true, '업로드 중...');
    try {
      await apiApproveBundle(b.id, caption.value.trim());
      showToast('인스타그램에 올렸어요!');
      await renderBundles();
    } catch (err: any) {
      showToast('업로드 실패: ' + err.message, true);
      setBusy(false, '승인하고 업로드');
    }
  });

  redo.addEventListener('click', async () => {
    if (!confirm('이 포스트를 버리고 다시 만들까요? (몇 분 걸려요)')) return;
    setBusy(true);
    try {
      await apiRegenerateBundle(b.id);
      showToast('다시 만드는 중이에요. 몇 분 뒤 새로고침 해주세요');
      await renderBundles();
    } catch (err: any) {
      showToast('요청 실패: ' + err.message, true);
      setBusy(false);
    }
  });
  return card;
}

export async function renderBundles(): Promise<void> {
  const list = document.getElementById('bundleList');
  if (!list) return;
  try {
    const bundles = await apiFetchBundles();
    list.replaceChildren();
    if (bundles.length === 0) {
      list.appendChild(el('div', 'opacity: 0.7; padding: 24px 0; text-align: center;', '승인 대기 중인 포스트가 없어요. 매일 아침 자동으로 만들어져요.'));
      return;
    }
    bundles.forEach(b => list.appendChild(renderBundle(b)));
  } catch (err: any) {
    showToast(err.message, true);
  }
}
