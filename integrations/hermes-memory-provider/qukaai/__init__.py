"""QukaAI memory provider for Hermes Agent.

This provider adapts Hermes' Python ``MemoryProvider`` lifecycle to the
QukaAI HTTP memory APIs. QukaAI remains the memory backend; Hermes only gets a
thin plugin that handles configuration, tool schemas, background prefetch, and
session lifecycle mapping.
"""

from __future__ import annotations

import json
import logging
import os
import threading
import urllib.error
import urllib.request
from pathlib import Path
from typing import Any, Dict, List, Optional

try:
    from agent.memory_provider import MemoryProvider
except Exception:  # pragma: no cover - lets local tests run outside Hermes.
    class MemoryProvider:  # type: ignore[no-redef]
        pass

try:
    from tools.registry import tool_error
except Exception:  # pragma: no cover - lets local tests run outside Hermes.
    def tool_error(message: str) -> str:
        return json.dumps({"error": message})


logger = logging.getLogger(__name__)

CONFIG_FILE = "qukaai-memory.json"
DEFAULT_TIMEOUT = 15.0

SEARCH_SCHEMA = {
    "name": "qukaai_memory_search",
    "description": (
        "Search QukaAI persistent agent memories. Use this for durable user "
        "preferences, prior project decisions, stable constraints, and memories "
        "the user expects the agent to recall. This searches agent memory, not "
        "the user's general knowledge base."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "query": {"type": "string", "description": "Memory search query."},
            "limit": {
                "type": "integer",
                "description": "Maximum memories to return. Default 6.",
            },
            "memory_types": {
                "type": "array",
                "items": {"type": "string", "enum": ["core", "semantic", "episodic", "working"]},
                "description": "Optional memory type filter.",
            },
            "entity_keys": {
                "type": "array",
                "items": {"type": "string"},
                "description": "Optional entity keys such as repo, project, person, or topic.",
            },
        },
        "required": ["query"],
    },
}

REMEMBER_SCHEMA = {
    "name": "qukaai_memory_remember",
    "description": (
        "Store a durable QukaAI agent memory. Do not use for documents, notes, "
        "references, raw logs, temporary task state, secrets, or content that "
        "belongs in the user's knowledge base."
    ),
    "parameters": {
        "type": "object",
        "properties": {
            "content": {"type": "string", "description": "Concise durable memory content."},
            "title": {"type": "string", "description": "Short title for the memory."},
            "layer": {
                "type": "string",
                "enum": ["user_global", "user_space", "space_shared"],
                "description": "Memory layer. Default is provider configuration.",
            },
            "memory_type": {
                "type": "string",
                "enum": ["core", "semantic", "episodic", "working"],
                "description": "Memory type. Default semantic.",
            },
            "entity_key": {"type": "string", "description": "Optional entity key."},
            "importance": {
                "type": "integer",
                "description": "Importance from 1 to 100. Default 70.",
            },
        },
        "required": ["content"],
    },
}

FORGET_SCHEMA = {
    "name": "qukaai_memory_forget",
    "description": "Forget an obsolete, incorrect, duplicated, or user-requested QukaAI memory.",
    "parameters": {
        "type": "object",
        "properties": {
            "memory_id": {"type": "string", "description": "Memory ID to forget."},
            "delete_knowledge": {
                "type": "boolean",
                "description": "Also delete backing hidden knowledge. Default true.",
            },
        },
        "required": ["memory_id"],
    },
}

HYDRATE_SCHEMA = {
    "name": "qukaai_memory_hydrate",
    "description": "Assemble QukaAI memory context for the current Hermes runtime context.",
    "parameters": {
        "type": "object",
        "properties": {
            "query": {"type": "string", "description": "Current user message or task."},
            "token_budget": {
                "type": "integer",
                "description": "Approximate token budget for assembled memory context.",
            },
        },
        "required": ["query"],
    },
}

PIN_SCHEMA = {
    "name": "qukaai_memory_pin",
    "description": "Pin QukaAI memories to the current Hermes runtime context as working memory.",
    "parameters": {
        "type": "object",
        "properties": {
            "memory_ids": {
                "type": "array",
                "items": {"type": "string"},
                "description": "Memory IDs to pin.",
            },
            "binding_type": {
                "type": "string",
                "enum": ["pin", "working", "hydration_cache"],
                "description": "Binding type. Default pin.",
            },
        },
        "required": ["memory_ids"],
    },
}


class QukaAIHTTPError(RuntimeError):
    """Raised when the QukaAI HTTP API returns an error."""


class QukaAIMemoryProvider(MemoryProvider):
    """Hermes MemoryProvider backed by QukaAI memory APIs."""

    def __init__(self, config: Optional[Dict[str, Any]] = None) -> None:
        self._config: Dict[str, Any] = dict(config or {})
        self._hermes_home = ""
        self._api_base_url = ""
        self._access_token = ""
        self._auth_token = ""
        self._space_id = ""
        self._session_id = ""
        self._runtime_context: Dict[str, Any] = {}
        self._agent_identity = "default"
        self._agent_workspace = "hermes"
        self._platform = "cli"
        self._agent_context = "primary"
        self._timeout = DEFAULT_TIMEOUT
        self._prefetch_thread: Optional[threading.Thread] = None
        self._sync_thread: Optional[threading.Thread] = None
        self._prefetch_lock = threading.Lock()
        self._prefetch_result = ""
        self._shutting_down = False

    @property
    def name(self) -> str:
        return "qukaai"

    def is_available(self) -> bool:
        """Check local config only; no network calls."""
        config = self._load_config_for_availability()
        api_base_url = os.environ.get("QUKA_API_BASE_URL") or str(config.get("api_base_url") or "")
        access_token = os.environ.get("QUKA_ACCESS_TOKEN") or str(config.get("access_token") or "")
        auth_token = os.environ.get("QUKA_AUTH_TOKEN") or str(config.get("auth_token") or "")
        space_id = os.environ.get("QUKA_SPACE_ID") or str(config.get("space_id") or "")
        return bool(api_base_url.strip() and space_id.strip() and (access_token.strip() or auth_token.strip()))

    def initialize(self, session_id: str, **kwargs: Any) -> None:
        self._hermes_home = str(kwargs.get("hermes_home") or "")
        saved = self._load_saved_config(self._hermes_home)
        merged = {**saved, **self._config}

        self._api_base_url = self._normalize_api_base_url(
            os.environ.get("QUKA_API_BASE_URL") or str(merged.get("api_base_url") or "")
        )
        self._access_token = os.environ.get("QUKA_ACCESS_TOKEN") or str(merged.get("access_token") or "")
        self._auth_token = os.environ.get("QUKA_AUTH_TOKEN") or str(merged.get("auth_token") or "")
        self._space_id = os.environ.get("QUKA_SPACE_ID") or str(merged.get("space_id") or "")
        self._timeout = float(merged.get("timeout_seconds") or DEFAULT_TIMEOUT)
        self._config = merged

        self._agent_identity = str(kwargs.get("agent_identity") or merged.get("agent_identity") or "default")
        self._agent_workspace = str(kwargs.get("agent_workspace") or merged.get("agent_workspace") or "hermes")
        self._platform = str(kwargs.get("platform") or merged.get("platform") or "cli")
        self._agent_context = str(kwargs.get("agent_context") or "primary")
        self._set_session(session_id)

        if not self._api_base_url:
            logger.warning("QukaAI memory provider initialized without api_base_url")
        if not self._space_id:
            logger.warning("QukaAI memory provider initialized without space_id")

    def system_prompt_block(self) -> str:
        return (
            "# QukaAI Memory\n"
            "QukaAI memory is active as persistent agent memory. It is separate "
            "from the user's knowledge base: use memory for durable runtime facts, "
            "preferences, corrections, project conventions, and long-lived decisions; "
            "use knowledge tools for documents, notes, references, and long-form material."
        )

    def queue_prefetch(self, query: str, *, session_id: str = "") -> None:
        if self._shutting_down or self._agent_context != "primary":
            return
        if session_id:
            self._set_session(session_id)
        query = (query or "").strip()
        if not query:
            return

        def _run() -> None:
            try:
                response = self._post_memory("hydrate", {
                    "runtime_context": self._runtime_context,
                    "query": query,
                    "token_budget": self._int_config("hydrate_token_budget", 1200),
                })
                text = str(response.get("assembled_context") or "").strip()
                with self._prefetch_lock:
                    self._prefetch_result = text
            except Exception as exc:
                logger.debug("QukaAI memory prefetch failed: %s", exc, exc_info=True)

        self._join_thread(self._prefetch_thread, timeout=1.0)
        self._prefetch_thread = threading.Thread(target=_run, daemon=True, name="qukaai-prefetch")
        self._prefetch_thread.start()

    def prefetch(self, query: str, *, session_id: str = "") -> str:
        if session_id:
            self._set_session(session_id)
        self._join_thread(self._prefetch_thread, timeout=3.0)
        with self._prefetch_lock:
            result = self._prefetch_result
            self._prefetch_result = ""
        if not result.strip():
            return ""
        return (
            "# QukaAI Memory\n"
            "Persistent agent memories relevant to this turn. Treat them as background context, "
            "not as new user input.\n\n"
            f"{result.strip()}"
        )

    def sync_turn(
        self,
        user_content: str,
        assistant_content: str,
        *,
        session_id: str = "",
        messages: Optional[List[Dict[str, Any]]] = None,
    ) -> None:
        if self._shutting_down or self._agent_context != "primary":
            return
        mode = str(self._config.get("sync_turn") or "off").lower()
        if mode not in {"reflect", "remember"}:
            return
        if session_id:
            self._set_session(session_id)

        def _run() -> None:
            try:
                if mode == "remember":
                    content = self._format_turn_memory(user_content, assistant_content)
                    if content:
                        self._remember(content, title="Hermes turn memory", memory_type="episodic")
                    return
                extraction = self._messages_extraction(messages) if messages else {
                    "messages": [
                        {"role": "user", "content": user_content or ""},
                        {"role": "assistant", "content": assistant_content or ""},
                    ]
                }
                self._reflect(extraction)
            except Exception as exc:
                logger.debug("QukaAI memory sync_turn failed: %s", exc, exc_info=True)

        self._join_thread(self._sync_thread, timeout=1.0)
        self._sync_thread = threading.Thread(target=_run, daemon=True, name="qukaai-sync")
        self._sync_thread.start()

    def on_session_end(self, messages: List[Dict[str, Any]]) -> None:
        if not self._bool_config("reflect_on_session_end", True):
            return
        if self._agent_context != "primary":
            return
        try:
            self._reflect(self._messages_extraction(messages))
        except Exception as exc:
            logger.debug("QukaAI memory on_session_end failed: %s", exc, exc_info=True)

    def on_pre_compress(self, messages: List[Dict[str, Any]]) -> str:
        if not self._bool_config("reflect_on_pre_compress", True):
            return ""
        if self._agent_context != "primary":
            return ""
        try:
            memory_id = self._reflect(self._messages_extraction(messages))
            if memory_id:
                return f"QukaAI stored compressed-session memory as memory_id={memory_id}."
        except Exception as exc:
            logger.debug("QukaAI memory on_pre_compress failed: %s", exc, exc_info=True)
        return ""

    def on_session_switch(
        self,
        new_session_id: str,
        *,
        parent_session_id: str = "",
        reset: bool = False,
        rewound: bool = False,
        **kwargs: Any,
    ) -> None:
        self._set_session(new_session_id)
        with self._prefetch_lock:
            self._prefetch_result = ""

    def on_memory_write(
        self,
        action: str,
        target: str,
        content: str,
        metadata: Optional[Dict[str, Any]] = None,
    ) -> None:
        if not self._bool_config("mirror_builtin_memory", True):
            return
        if self._agent_context != "primary":
            return
        action = (action or "").lower()
        target = (target or "memory").lower()
        metadata = metadata or {}
        layer = "user_global" if target == "user" else str(self._config.get("default_layer") or "user_space")
        if action in {"add", "replace"} and content.strip():
            self._remember(
                content.strip(),
                title=f"Hermes built-in {target} memory",
                layer=layer,
                memory_type="core" if target == "user" else "semantic",
                entity_key=f"hermes_builtin:{target}",
                source_ref=str(metadata.get("session_id") or self._session_id),
            )
        elif action == "remove" and content.strip():
            try:
                items = self._post_memory("recall", {"query": content.strip(), "limit": 5}).get("items") or []
                for item in items:
                    if str(item.get("content") or "").strip() == content.strip():
                        memory_id = str(item.get("memory_id") or "")
                        if memory_id:
                            self._post_memory("delete", {"id": memory_id, "hard": True, "delete_knowledge": True})
                            break
            except Exception as exc:
                logger.debug("QukaAI memory built-in remove mirror failed: %s", exc, exc_info=True)

    def on_delegation(self, task: str, result: str, *, child_session_id: str = "", **kwargs: Any) -> None:
        if not self._bool_config("remember_delegation_results", False):
            return
        content = self._trim(
            f"Delegated task: {task}\n\nResult from child session {child_session_id}: {result}",
            self._int_config("delegation_max_chars", 4000),
        )
        if content:
            self._remember(content, title="Hermes delegation result", memory_type="episodic")

    def get_tool_schemas(self) -> List[Dict[str, Any]]:
        schemas = [SEARCH_SCHEMA, REMEMBER_SCHEMA, FORGET_SCHEMA, HYDRATE_SCHEMA, PIN_SCHEMA]
        if not self._bool_config("allow_space_shared_tool", False):
            schemas = json.loads(json.dumps(schemas))
            for schema in schemas:
                layer = schema.get("parameters", {}).get("properties", {}).get("layer")
                if layer and "enum" in layer:
                    layer["enum"] = ["user_global", "user_space"]
        return schemas

    def handle_tool_call(self, tool_name: str, args: Dict[str, Any], **kwargs: Any) -> str:
        try:
            if tool_name == "qukaai_memory_search":
                response = self._post_memory("recall", {
                    "query": str(args.get("query") or ""),
                    "limit": int(args.get("limit") or self._int_config("prefetch_limit", 6)),
                    "memory_types": args.get("memory_types") or [],
                    "entity_keys": args.get("entity_keys") or [],
                })
                return json.dumps({"items": response.get("items") or [], "count": response.get("count") or 0})

            if tool_name == "qukaai_memory_remember":
                content = str(args.get("content") or "").strip()
                if not content:
                    return tool_error("Missing required parameter: content")
                response = self._remember(
                    content,
                    title=str(args.get("title") or ""),
                    layer=str(args.get("layer") or self._config.get("default_layer") or "user_space"),
                    memory_type=str(args.get("memory_type") or "semantic"),
                    entity_key=str(args.get("entity_key") or ""),
                    importance=int(args.get("importance") or 70),
                )
                return json.dumps(response)

            if tool_name == "qukaai_memory_forget":
                memory_id = str(args.get("memory_id") or "")
                if not memory_id:
                    return tool_error("Missing required parameter: memory_id")
                delete_knowledge = bool(args.get("delete_knowledge", True))
                self._post_memory("delete", {"id": memory_id, "hard": True, "delete_knowledge": delete_knowledge})
                return json.dumps({"result": "Memory forgotten.", "memory_id": memory_id})

            if tool_name == "qukaai_memory_hydrate":
                response = self._post_memory("hydrate", {
                    "runtime_context": self._runtime_context,
                    "query": str(args.get("query") or ""),
                    "token_budget": int(args.get("token_budget") or self._int_config("hydrate_token_budget", 1200)),
                })
                return json.dumps(response)

            if tool_name == "qukaai_memory_pin":
                memory_ids = args.get("memory_ids") or []
                if not isinstance(memory_ids, list) or not memory_ids:
                    return tool_error("Missing required parameter: memory_ids")
                self._post_memory("pin", {
                    "runtime_context": self._runtime_context,
                    "memory_ids": memory_ids,
                    "binding_type": str(args.get("binding_type") or "pin"),
                    "pinned_by": "agent",
                })
                return json.dumps({"result": "Memories pinned.", "memory_ids": memory_ids})
        except Exception as exc:
            logger.debug("QukaAI memory tool %s failed: %s", tool_name, exc, exc_info=True)
            return tool_error(str(exc))

        return tool_error(f"Unknown QukaAI memory tool: {tool_name}")

    def get_config_schema(self) -> List[Dict[str, Any]]:
        return [
            {
                "key": "api_base_url",
                "description": "QukaAI API base URL, for example http://localhost:8080/api/v1",
                "required": True,
                "default": os.environ.get("QUKA_API_BASE_URL", ""),
            },
            {
                "key": "access_token",
                "description": "QukaAI access token for X-Access-Token",
                "secret": True,
                "required": False,
                "env_var": "QUKA_ACCESS_TOKEN",
            },
            {
                "key": "auth_token",
                "description": "QukaAI auth token for X-Authorization",
                "secret": True,
                "required": False,
                "env_var": "QUKA_AUTH_TOKEN",
            },
            {
                "key": "space_id",
                "description": "Default QukaAI space ID for Hermes memory operations",
                "required": True,
                "default": os.environ.get("QUKA_SPACE_ID", ""),
            },
            {
                "key": "default_layer",
                "description": "Default memory layer",
                "default": "user_space",
                "choices": ["user_space", "user_global", "space_shared"],
            },
        ]

    def save_config(self, values: Dict[str, Any], hermes_home: str) -> None:
        clean = {k: v for k, v in values.items() if k not in {"access_token", "auth_token"}}
        path = Path(hermes_home) / CONFIG_FILE
        path.write_text(json.dumps(clean, indent=2, sort_keys=True), encoding="utf-8")

    def shutdown(self) -> None:
        self._shutting_down = True
        self._join_thread(self._prefetch_thread, timeout=5.0)
        self._join_thread(self._sync_thread, timeout=5.0)

    def _remember(
        self,
        content: str,
        *,
        title: str = "",
        layer: str = "",
        memory_type: str = "semantic",
        entity_key: str = "",
        importance: int = 70,
        source_ref: str = "",
    ) -> Dict[str, Any]:
        payload = {
            "title": title,
            "content": content,
            "content_type": "markdown",
            "kind": "text",
            "layer": layer or str(self._config.get("default_layer") or "user_space"),
            "memory_type": memory_type or "semantic",
            "author_type": "agent",
            "epistemic_status": "observed",
            "entity_key": entity_key,
            "importance": importance,
            "confidence": 0.85,
            "source_kind": "chat",
            "source_ref": f"hermes:{source_ref or self._session_id}",
        }
        return self._post_memory("remember", payload)

    def _reflect(self, extraction: Dict[str, Any]) -> str:
        response = self._post_memory("reflect", {
            "runtime_context": {**self._runtime_context, "extraction": extraction},
            "mode": "hermes",
        })
        return str(response.get("memory_id") or "")

    def _post_memory(self, action: str, payload: Dict[str, Any]) -> Dict[str, Any]:
        if not self._space_id:
            raise QukaAIHTTPError("QukaAI space_id is not configured")
        return self._request("POST", f"/{self._space_id}/memory/{action}", payload)

    def _request(self, method: str, path: str, payload: Optional[Dict[str, Any]] = None) -> Dict[str, Any]:
        if not self._api_base_url:
            raise QukaAIHTTPError("QukaAI api_base_url is not configured")
        url = f"{self._api_base_url}{path}"
        body = json.dumps(payload or {}).encode("utf-8")
        headers = {
            "Content-Type": "application/json",
            "Accept": "application/json",
            "X-Client-Source": "hermes-agent",
        }
        if self._access_token:
            headers["X-Access-Token"] = self._access_token
        if self._auth_token:
            headers["X-Authorization"] = self._auth_token

        request = urllib.request.Request(url, data=body, headers=headers, method=method)
        try:
            with urllib.request.urlopen(request, timeout=self._timeout) as response:
                raw = response.read().decode("utf-8")
        except urllib.error.HTTPError as exc:
            detail = exc.read().decode("utf-8", errors="replace")
            raise QukaAIHTTPError(f"QukaAI HTTP {exc.code}: {detail}") from exc
        except urllib.error.URLError as exc:
            raise QukaAIHTTPError(f"QukaAI request failed: {exc}") from exc

        if not raw.strip():
            return {}
        decoded = json.loads(raw)
        if isinstance(decoded, dict) and "meta" in decoded:
            meta = decoded.get("meta") or {}
            code = int(meta.get("code") or 0)
            if code >= 400:
                raise QukaAIHTTPError(str(meta.get("message") or f"QukaAI API error {code}"))
            data = decoded.get("data")
            return data if isinstance(data, dict) else {"data": data}
        return decoded if isinstance(decoded, dict) else {"data": decoded}

    def _set_session(self, session_id: str) -> None:
        self._session_id = session_id or self._session_id or "default"
        context_id = f"hermes:{self._agent_identity}:{self._platform}:{self._session_id}"
        self._runtime_context = {"type": "agent_run", "id": context_id}

    def _messages_extraction(self, messages: Optional[List[Dict[str, Any]]]) -> Dict[str, Any]:
        clean_messages: List[Dict[str, Any]] = []
        for msg in messages or []:
            if not isinstance(msg, dict):
                continue
            clean = {
                "role": str(msg.get("role") or ""),
                "content": self._message_content_to_text(msg.get("content")),
            }
            name = msg.get("name")
            tool_call_id = msg.get("tool_call_id")
            if name:
                clean["name"] = str(name)
            if tool_call_id:
                clean["tool_call_id"] = str(tool_call_id)
            clean_messages.append(clean)
        return {
            "messages": clean_messages,
            "metadata": {
                "provider": "qukaai",
                "hermes_session_id": self._session_id,
                "agent_identity": self._agent_identity,
                "agent_workspace": self._agent_workspace,
                "platform": self._platform,
            },
        }

    def _format_turn_memory(self, user_content: str, assistant_content: str) -> str:
        content = f"User: {(user_content or '').strip()}\nAssistant: {(assistant_content or '').strip()}".strip()
        return self._trim(content, self._int_config("sync_turn_max_chars", 4000))

    def _message_content_to_text(self, content: Any) -> str:
        if isinstance(content, str):
            return self._trim(content, self._int_config("message_max_chars", 4000))
        if isinstance(content, list):
            parts = []
            for item in content:
                if isinstance(item, dict):
                    text = item.get("text") or item.get("content")
                    if text:
                        parts.append(str(text))
                elif item:
                    parts.append(str(item))
            return self._trim("\n".join(parts), self._int_config("message_max_chars", 4000))
        if content is None:
            return ""
        return self._trim(str(content), self._int_config("message_max_chars", 4000))

    def _load_config_for_availability(self) -> Dict[str, Any]:
        try:
            from hermes_constants import get_hermes_home
            return self._load_saved_config(str(get_hermes_home()))
        except Exception:
            return {}

    def _load_saved_config(self, hermes_home: str) -> Dict[str, Any]:
        if not hermes_home:
            return {}
        path = Path(hermes_home) / CONFIG_FILE
        if not path.exists():
            return {}
        try:
            raw = path.read_text(encoding="utf-8")
            data = json.loads(raw)
            return data if isinstance(data, dict) else {}
        except Exception as exc:
            logger.debug("Failed to load QukaAI memory config %s: %s", path, exc)
            return {}

    @staticmethod
    def _normalize_api_base_url(value: str) -> str:
        value = (value or "").strip().rstrip("/")
        if not value:
            return ""
        if value.endswith("/api/v1"):
            return value
        return f"{value}/api/v1"

    @staticmethod
    def _join_thread(thread: Optional[threading.Thread], timeout: float) -> None:
        if thread and thread.is_alive():
            thread.join(timeout=timeout)

    @staticmethod
    def _trim(text: str, max_chars: int) -> str:
        text = text or ""
        if max_chars <= 0 or len(text) <= max_chars:
            return text
        return text[:max_chars].rstrip()

    def _int_config(self, key: str, default: int) -> int:
        try:
            return int(self._config.get(key) or default)
        except Exception:
            return default

    def _bool_config(self, key: str, default: bool) -> bool:
        value = self._config.get(key, default)
        if isinstance(value, bool):
            return value
        if isinstance(value, str):
            return value.lower() not in {"0", "false", "no", "off"}
        return bool(value)


def register(ctx: Any) -> None:
    """Register QukaAI as a Hermes memory provider plugin."""
    ctx.register_memory_provider(QukaAIMemoryProvider())

