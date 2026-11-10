import type { Draft, StyleItem } from './types';

export class AppState {
  styles: StyleItem[] = [];
  selectedStyle: StyleItem | null = null;
  currentEra: string = 'ALL';

  drafts: Draft[] = [];
  baseDraft: Draft | null = null;
  carouselDrafts: Draft[] = [];

  activeFeedbackTags: string[] = [];
  selectedAngles: string[] = [
    'Full-body candid runway walk, head to toe',
    'Extreme close-up on fabric texture, stitching and accessories'
  ];
  customAngles: { title: string; desc: string; prompt: string }[] = [];

  currentStep: number = 0;

  setStyles(list: StyleItem[]) {
    this.styles = list;
    if (!this.selectedStyle && list.length > 0) {
      this.selectedStyle = list[0];
    }
  }

  setDrafts(list: Draft[]) {
    this.drafts = list;
    if (!this.baseDraft && list.length > 0) {
      this.baseDraft = list[0];
      this.carouselDrafts = [list[0]];
    } else if (list.length === 0) {
      this.baseDraft = null;
      this.carouselDrafts = [];
    }
  }

  setBaseDraft(draft: Draft) {
    this.baseDraft = draft;
    if (!this.carouselDrafts.some(d => d.id === draft.id)) {
      this.carouselDrafts = [draft, ...this.carouselDrafts];
    }
  }
}

export const state = new AppState();
