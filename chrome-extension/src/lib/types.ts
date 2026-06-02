import type { PartialBlock } from '@blocknote/core';

export type AuthType = 'access' | 'login';

export interface ExtensionSettings {
  host: string;
  authType: AuthType;
  token: string;
  selectedSpaceId: string;
  selectedResourceId: string;
  selectedChatSessionId: string;
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

export interface ChatSession {
  id: string;
  space_id: string;
  user_id: string;
  title: string;
  session_type: number;
  status: number;
  created_at: number;
  latest_access_time: number;
}

export interface ChatMessageMeta {
  message_id: string;
  sequence: number;
  send_time: number;
  role: number;
  user_id: string;
  session_id: string;
  space_id: string;
  complete: number;
  message_type: number;
  message?: {
    text?: string;
  };
  attach?: Array<{
    type: string;
    url: string;
    ai_desc?: string;
  }>;
}

export interface ChatMessageDetail {
  meta: ChatMessageMeta;
  ext?: {
    rel_docs?: Array<{
      id: string;
      title: string;
      resource: string;
      space_id: string;
    }>;
    evaluate?: number;
    tool_name?: string;
    tool_args?: string;
    is_evaluate_enable?: boolean;
  } | null;
}

export interface ChatHistoryResponse {
  list: ChatMessageDetail[];
  total: number;
}

export interface ChatSendResult {
  sequence: number;
  answer_id: string;
}

export type MemoryBlocks = PartialBlock[];
