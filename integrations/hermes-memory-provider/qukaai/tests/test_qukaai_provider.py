from __future__ import annotations

import importlib.util
import json
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch


MODULE_PATH = Path(__file__).resolve().parents[1] / "__init__.py"
SPEC = importlib.util.spec_from_file_location("qukaai_memory_provider", MODULE_PATH)
qukaai = importlib.util.module_from_spec(SPEC)
assert SPEC and SPEC.loader
SPEC.loader.exec_module(qukaai)


class QukaAIMemoryProviderTest(unittest.TestCase):
    def test_save_config_omits_secret_tokens(self) -> None:
        provider = qukaai.QukaAIMemoryProvider()
        with tempfile.TemporaryDirectory() as tmp:
            provider.save_config(
                {
                    "api_base_url": "http://localhost:8080/api/v1",
                    "space_id": "space_1",
                    "access_token": "secret",
                    "auth_token": "secret2",
                },
                tmp,
            )
            data = json.loads((Path(tmp) / "qukaai-memory.json").read_text())

        self.assertEqual(data["space_id"], "space_1")
        self.assertNotIn("access_token", data)
        self.assertNotIn("auth_token", data)

    def test_initialize_reads_config_and_builds_runtime_context(self) -> None:
        provider = qukaai.QukaAIMemoryProvider()
        with tempfile.TemporaryDirectory() as tmp:
            (Path(tmp) / "qukaai-memory.json").write_text(
                json.dumps({"api_base_url": "http://quka.local", "space_id": "space_1"}),
                encoding="utf-8",
            )
            with patch.dict(os.environ, {"QUKA_ACCESS_TOKEN": "token"}, clear=False):
                provider.initialize(
                    "session_1",
                    hermes_home=tmp,
                    platform="cli",
                    agent_identity="coder",
                )

        self.assertEqual(provider._api_base_url, "http://quka.local/api/v1")
        self.assertEqual(provider._space_id, "space_1")
        self.assertEqual(provider._runtime_context["type"], "agent_run")
        self.assertEqual(provider._runtime_context["id"], "hermes:coder:cli:session_1")

    def test_tool_schema_hides_space_shared_by_default(self) -> None:
        provider = qukaai.QukaAIMemoryProvider()
        remember = next(schema for schema in provider.get_tool_schemas() if schema["name"] == "qukaai_memory_remember")
        layer_enum = remember["parameters"]["properties"]["layer"]["enum"]

        self.assertEqual(layer_enum, ["user_global", "user_space"])

    def test_tool_schema_allows_space_shared_when_configured(self) -> None:
        provider = qukaai.QukaAIMemoryProvider({"allow_space_shared_tool": True})
        remember = next(schema for schema in provider.get_tool_schemas() if schema["name"] == "qukaai_memory_remember")
        layer_enum = remember["parameters"]["properties"]["layer"]["enum"]

        self.assertIn("space_shared", layer_enum)

    def test_handle_remember_maps_to_quka_memory_api(self) -> None:
        provider = qukaai.QukaAIMemoryProvider({"api_base_url": "http://quka.local/api/v1", "space_id": "space_1"})
        provider.initialize("session_1", platform="cli", agent_identity="coder")
        calls = []

        def fake_post(action, payload):
            calls.append((action, payload))
            return {"memory_id": "mem_1", "knowledge_id": "know_1"}

        provider._post_memory = fake_post
        result = json.loads(
            provider.handle_tool_call(
                "qukaai_memory_remember",
                {
                    "content": "User prefers concise answers.",
                    "title": "Answer style",
                    "layer": "user_global",
                    "memory_type": "core",
                    "entity_key": "preference:style",
                },
            )
        )

        self.assertEqual(result["memory_id"], "mem_1")
        self.assertEqual(calls[0][0], "remember")
        payload = calls[0][1]
        self.assertEqual(payload["layer"], "user_global")
        self.assertEqual(payload["memory_type"], "core")
        self.assertEqual(payload["author_type"], "agent")
        self.assertEqual(payload["source_ref"], "hermes:session_1")

    def test_response_envelope_is_unwrapped(self) -> None:
        provider = qukaai.QukaAIMemoryProvider({"api_base_url": "http://quka.local/api/v1", "space_id": "space_1"})
        provider.initialize("session_1")

        class FakeResponse:
            def __enter__(self):
                return self

            def __exit__(self, exc_type, exc, tb):
                return False

            def read(self):
                return json.dumps({"meta": {"code": 0}, "data": {"items": [1]}}).encode()

        with patch("urllib.request.urlopen", return_value=FakeResponse()):
            data = provider._request("POST", "/space_1/memory/recall", {"query": "x"})

        self.assertEqual(data, {"items": [1]})

    def test_reflect_uses_extraction_in_runtime_context(self) -> None:
        provider = qukaai.QukaAIMemoryProvider({"api_base_url": "http://quka.local/api/v1", "space_id": "space_1"})
        provider.initialize("session_1", platform="cli", agent_identity="coder")
        calls = []

        def fake_post(action, payload):
            calls.append((action, payload))
            return {"memory_id": "mem_reflect"}

        provider._post_memory = fake_post
        result = provider.on_pre_compress([{"role": "user", "content": "Remember the migration plan."}])

        self.assertIn("mem_reflect", result)
        self.assertEqual(calls[0][0], "reflect")
        runtime_context = calls[0][1]["runtime_context"]
        self.assertEqual(runtime_context["type"], "agent_run")
        self.assertIn("extraction", runtime_context)
        self.assertEqual(runtime_context["extraction"]["messages"][0]["role"], "user")


if __name__ == "__main__":
    unittest.main()

