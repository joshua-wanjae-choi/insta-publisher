import type { Draft, StyleItem, GenerateJobPayload } from './types';

export function getImageUrl(draft: Draft | null | undefined): string {
  if (!draft) return '';
  if (draft.file_name) return `/media/${draft.file_name}`;
  if (draft.preview_url) {
    if (draft.preview_url.startsWith('http')) {
      try {
        return new URL(draft.preview_url).pathname;
      } catch {
        return draft.preview_url;
      }
    }
    return draft.preview_url;
  }
  return '';
}

export async function apiFetchStyles(): Promise<StyleItem[]> {
  const res = await fetch('/api/styles');
  if (!res.ok) throw new Error('사조 목록을 불러오지 못했습니다');
  const data = await res.json();
  return Array.isArray(data) ? data : [];
}

export async function apiFetchDrafts(): Promise<Draft[]> {
  const res = await fetch('/api/drafts');
  if (!res.ok) throw new Error('화보 목록을 불러오지 못했습니다');
  const data = await res.json();
  return Array.isArray(data) ? data : [];
}

export async function apiDeleteDraft(draftId: string): Promise<void> {
  const res = await fetch(`/api/drafts/${draftId}`, { method: 'DELETE' });
  if (!res.ok) throw new Error('화보 삭제에 실패했습니다');
}

export async function apiSendGenerateJob(payload: GenerateJobPayload): Promise<void> {
  const res = await fetch('/api/generate-request', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(payload)
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `서버 오류 (${res.status})`);
  }
}

export async function apiPublishCarousel(ids: string[], caption: string): Promise<{ status: string; media_id: string }> {
  const res = await fetch('/api/publish', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ ids, caption })
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({}));
    throw new Error(err.error || `발행 실패 (${res.status})`);
  }
  return res.json();
}
