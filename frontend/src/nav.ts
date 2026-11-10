import { state } from './state';
import { updateBasePreview, renderGallery } from './step2';
import { renderAngleGrid, renderVariationCandidates } from './step3';
import { renderFinalThumbs } from './step4';
import { renderBundles } from './today';

export function switchStep(step: number) {
  state.currentStep = step;

  // 탭 인디케이터 활성화
  document.querySelectorAll('.step-tab').forEach((tab, idx) => {
    if (idx === step) {
      tab.classList.add('active');
    } else {
      tab.classList.remove('active');
    }
  });

  // 콘텐츠 영역 가시성 전환
  for (let i = 0; i <= 4; i++) {
    const el = document.getElementById(`step-content-${i}`);
    if (el) {
      el.style.display = (i === step) ? 'block' : 'none';
    }
  }

  // 각 스텝 전환 시 필요한 뷰 리프레시
  if (step === 0) {
    renderBundles();
  } else if (step === 2) {
    updateBasePreview();
    renderGallery();
  } else if (step === 3) {
    renderAngleGrid();
    renderVariationCandidates();
  } else if (step === 4) {
    renderFinalThumbs();
  }
}
