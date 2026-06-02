import md5 from 'crypto-js/md5';

import { sleep } from './utils';
import type {
  AuthType,
  ChatHistoryResponse,
  ChatSendResult,
  ChatSession,
  ExtensionSettings,
  MemoryBlocks,
  QukaUser,
  Resource,
  SummaryResult,
  UserSpace
} from './types';

interface ApiResponse<T> {
  meta?: {
    code?: number;
    message?: string;
    request_id?: string;
  };
  data: T;
}

interface LoginResponse {
  meta: QukaUser;
  token: string;
  expire_at: number;
}

export function normalizeHost(host: string) {
  const trimmed = host.trim().replace(/\/+$/, '');

  if (!trimmed) {
    return '';
  }

  const parsed = new URL(trimmed);
  const path = parsed.pathname.replace(/\/+$/, '');
  parsed.search = '';
  parsed.hash = '';

  if (path.endsWith('/api/v1')) {
    return parsed.toString().replace(/\/+$/, '');
  }

  parsed.pathname = `${path === '/' ? '' : path}/api/v1`;

  return parsed.toString().replace(/\/+$/, '');
}

export function authHeaders(authType: AuthType, token: string) {
  if (!token) {
    return {};
  }

  return authType === 'access' ? { 'X-Access-Token': token } : { 'X-Authorization': token };
}

async function request<T>(settings: ExtensionSettings, path: string, init: RequestInit = {}, withAuth = true): Promise<T> {
  const host = normalizeHost(settings.host);
  const headers = new Headers(init.headers);

  headers.set('Content-Type', 'application/json');
  headers.set('Accept-Language', 'zh-CN');

  if (withAuth) {
    Object.entries(authHeaders(settings.authType, settings.token)).forEach(([key, value]) => {
      headers.set(key, value);
    });
  }

  const response = await fetch(`${host}${path}`, {
    ...init,
    headers
  });
  const payload = (await response.json().catch(() => null)) as ApiResponse<T> | null;

  if (!response.ok) {
    throw new Error(payload?.meta?.message || `${response.status} ${response.statusText}`);
  }

  if (payload?.meta?.code && payload.meta.code >= 400) {
    throw new Error(payload.meta.message || 'Quka API request failed.');
  }

  return payload?.data as T;
}

export async function loginWithAccessToken(settings: ExtensionSettings, accessToken: string): Promise<QukaUser> {
  return request<QukaUser>(
    {
      ...settings,
      authType: 'access',
      token: accessToken
    },
    '/user/info'
  );
}

export async function loginWithPassword(settings: ExtensionSettings, email: string, password: string): Promise<LoginResponse> {
  return request<LoginResponse>(
    settings,
    '/login',
    {
      method: 'POST',
      body: JSON.stringify({
        email,
        password: md5(password).toString()
      })
    },
    false
  );
}

export async function getUser(settings: ExtensionSettings) {
  return request<QukaUser>(settings, '/user/info');
}

export async function listSpaces(settings: ExtensionSettings) {
  const data = await request<{ list: UserSpace[] }>(settings, '/space/list');
  return data.list || [];
}

export async function listResources(settings: ExtensionSettings, spaceId: string) {
  const data = await request<{ list: Resource[] }>(settings, `/${spaceId}/resource/list`);
  return data.list || [];
}

export async function listChatSessions(settings: ExtensionSettings, spaceId: string, page = 1, pageSize = 20) {
  const data = await request<{ list: ChatSession[]; total: number }>(settings, `/${spaceId}/chat/list?page=${page}&pagesize=${pageSize}`);
  return {
    list: data.list || [],
    total: data.total || 0
  };
}

export async function createChatSession(settings: ExtensionSettings, spaceId: string) {
  const data = await request<{ session_id: string }>(settings, `/${spaceId}/chat`, {
    method: 'POST'
  });

  return data.session_id;
}

export async function getChatHistory(settings: ExtensionSettings, spaceId: string, sessionId: string, page = 1, pageSize = 50, afterSequence = 0) {
  return request<ChatHistoryResponse>(settings, `/${spaceId}/chat/${sessionId}/history/list?page=${page}&pagesize=${pageSize}&after_sequence=${afterSequence}`);
}

interface SendChatOptions {
  agent?: string;
  enableThinking?: boolean;
  enableSearch?: boolean;
  enableKnowledge?: boolean;
}

export async function sendChatMessage(
  settings: ExtensionSettings,
  spaceId: string,
  sessionId: string,
  message: string,
  options: SendChatOptions = {}
): Promise<ChatSendResult & { messageId: string }> {
  const messageId = await request<string>(settings, `/${spaceId}/chat/${sessionId}/message/id`, {
    method: 'POST'
  });
  const sendResult = await request<ChatSendResult>(settings, `/${spaceId}/chat/${sessionId}/message`, {
    method: 'POST',
    body: JSON.stringify({
      message_id: messageId,
      message,
      agent: options.agent || '',
      files: [],
      enable_thinking: options.enableThinking ?? false,
      enable_search: options.enableSearch ?? false,
      enable_knowledge: options.enableKnowledge ?? true
    })
  });

  return {
    ...sendResult,
    messageId
  };
}

export async function waitForChatAnswer(
  settings: ExtensionSettings,
  spaceId: string,
  sessionId: string,
  answerId: string,
  onHistory?: (history: ChatHistoryResponse) => void
) {
  const startedAt = Date.now();
  let latest = '';

  while (Date.now() - startedAt < 90000) {
    const history = await getChatHistory(settings, spaceId, sessionId, 1, 50, 0);
    onHistory?.(history);

    const answer = history.list.find(item => item.meta.message_id === answerId);

    if (answer) {
      latest = answer.meta.message?.text || latest;

      if (answer.meta.complete === 1) {
        return latest;
      }

      if ([4, 5, 6, 7].includes(answer.meta.complete)) {
        throw new Error(latest || 'QukaAI response did not complete.');
      }
    }

    await sleep(1500);
  }

  if (latest) {
    return latest;
  }

  throw new Error('Timed out waiting for QukaAI response.');
}

export async function sendChatMessageAndWait(
  settings: ExtensionSettings,
  spaceId: string,
  sessionId: string,
  message: string,
  options: SendChatOptions = {},
  onHistory?: (history: ChatHistoryResponse) => void
) {
  const sendResult = await sendChatMessage(settings, spaceId, sessionId, message, options);
  const text = await waitForChatAnswer(settings, spaceId, sessionId, sendResult.answer_id, onHistory);

  return {
    ...sendResult,
    text
  };
}

export async function createKnowledge(settings: ExtensionSettings, spaceId: string, resource: string, blocks: MemoryBlocks) {
  const data = await request<{ id: string }>(settings, `/${spaceId}/knowledge`, {
    method: 'POST',
    body: JSON.stringify({
      resource: resource || 'knowledge',
      kind: 'text',
      content: blocks,
      content_type: 'blocks_v2',
      async: true
    })
  });

  return data.id;
}

export async function summarizePage(
  settings: ExtensionSettings,
  spaceId: string,
  page: {
    title: string;
    url: string;
    description: string;
    selection: string;
    text: string;
  }
): Promise<SummaryResult> {
  const sessionId = await createChatSession(settings, spaceId);
  const message = buildSummaryPrompt(page);
  const sendResult = await sendChatMessageAndWait(settings, spaceId, sessionId, message, {
    enableThinking: false,
    enableSearch: false,
    enableKnowledge: false
  });

  return {
    sessionId,
    messageId: sendResult.messageId,
    answerId: sendResult.answer_id,
    text: sendResult.text
  };
}

function buildSummaryPrompt(page: { title: string; url: string; description: string; selection: string; text: string }) {
  const content = page.selection || page.text;

  return [
    '请总结当前网页内容，使用中文输出。',
    '要求：',
    '1. 先给出 3-5 条要点。',
    '2. 再给出一段 120 字以内的整体摘要。',
    '3. 如果原文有明确行动项、数据、结论或警示，请单独列出。',
    '4. 不要编造网页中没有的信息。',
    '',
    `标题：${page.title || '无标题'}`,
    `URL：${page.url}`,
    page.description ? `描述：${page.description}` : '',
    '',
    '网页内容：',
    content || '[empty page content]'
  ]
    .filter(Boolean)
    .join('\n');
}
