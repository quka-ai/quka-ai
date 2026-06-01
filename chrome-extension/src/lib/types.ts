import type { PartialBlock } from '@blocknote/core';

export type AuthType = 'access' | 'login';

export interface ExtensionSettings {
  host: string;
  authType: AuthType;
  token: string;
  selectedSpaceId: string;
  selectedResourceId: string;
}

export interface QukaUser {
  email: string;
  user_name: string;
  user_id: string;
  avatar: string;
  service_mode: string;
  plan_id: string;
  appid: string;
  system_role: string;
}

export interface UserSpace {
  user_id: string;
  space_id: string;
  role: string;
  title: string;
  description: string;
  base_prompt: string;
  chat_prompt: string;
  created_at: number;
}

export interface Resource {
  id: string;
  title: string;
  space_id: string;
  description: string;
  tag: string;
  cycle: number;
  created_at: number;
}

export interface PageSnapshot {
  title: string;
  url: string;
  description: string;
  selection: string;
  text: string;
  lang: string;
}

export interface SummaryResult {
  sessionId: string;
  messageId: string;
  answerId: string;
  text: string;
}

export type MemoryBlocks = PartialBlock[];
