# QukaAI Hermes Memory Provider

This plugin lets Hermes Agent use QukaAI as an external memory provider.

QukaAI keeps `memory` and `knowledge` separate:

- `memory` is persistent agent runtime memory: durable preferences, corrections, project conventions, decisions, pinned working context, and reflected session facts.
- `knowledge` is the user's searchable knowledge base: documents, notes, references, resources, and long-form material.

The plugin writes agent memories through QukaAI `/memory/*` APIs. Memory content created by the memory API uses QukaAI's hidden `__memory__` backing resource, so it does not pollute the normal user knowledge list.

## Install

Copy or symlink this directory into your Hermes profile:

```bash
mkdir -p "$HERMES_HOME/plugins"
ln -s /path/to/quka-ai/integrations/hermes-memory-provider/qukaai "$HERMES_HOME/plugins/qukaai"
```

Then enable it:

```yaml
memory:
  provider: qukaai
```

You can also select it through Hermes provider plugin setup if your Hermes build exposes the memory setup UI.

## Configuration

Required values:

- `QUKA_API_BASE_URL`: QukaAI API base URL, for example `http://localhost:8080/api/v1`.
- `QUKA_SPACE_ID`: QukaAI space ID used for Hermes memory.
- `QUKA_ACCESS_TOKEN`: QukaAI access token for `X-Access-Token`, or `QUKA_AUTH_TOKEN` for `X-Authorization`.

Non-secret settings are stored in:

```text
$HERMES_HOME/qukaai-memory.json
```

Example:

```json
{
  "api_base_url": "http://localhost:8080/api/v1",
  "space_id": "your_space_id",
  "default_layer": "user_space",
  "hydrate_token_budget": 1200,
  "prefetch_limit": 6,
  "sync_turn": "off",
  "reflect_on_session_end": true,
  "mirror_builtin_memory": true
}
```

## Hermes Mapping

- `queue_prefetch` calls QukaAI `/memory/hydrate` in a background thread.
- `prefetch` returns cached hydrated context to Hermes.
- `qukaai_memory_search` maps to `/memory/recall`.
- `qukaai_memory_remember` maps to `/memory/remember`.
- `qukaai_memory_forget` maps to `/memory/delete`.
- `qukaai_memory_hydrate` maps to `/memory/hydrate`.
- `qukaai_memory_pin` maps to `/memory/pin`.
- `on_session_end` and `on_pre_compress` map to `/memory/reflect`.

The runtime context ID uses:

```text
hermes:<agent_identity>:<platform>:<session_id>
```

## Safety Defaults

- The default memory layer is `user_space`.
- `space_shared` is hidden from the remember tool schema unless `allow_space_shared_tool` is enabled.
- `sync_turn` defaults to `off` to avoid storing noisy turn-by-turn transcripts.
- Built-in Hermes memory writes are mirrored by default when Hermes calls `on_memory_write`.

