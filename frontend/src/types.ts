export interface Draft {
  id: string;
  file_name: string;
  preview_url: string;
  caption: string;
  style_id?: string;
  created_at: string;
}

export interface StyleItem {
  id: string;
  era: string;
  name: string;
  keywords?: string;
  desc: string;
}

export interface AnglePreset {
  title: string;
  desc: string;
  prompt: string;
}

export interface GenerateJobPayload {
  type: 'initial' | 'refine' | 'variations';
  style_id: string;
  style_name: string;
  custom_prompt?: string;
  base_image_url?: string;
  feedback?: string;
  preset_angles?: string[];
  candidate_count?: number;
}
