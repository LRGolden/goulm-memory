# System Prompts for goulm-memory

Because `goulm-memory` is an explicit engine, agents will not use it automatically. You must inject instructions into your agent's **System Prompt** so the LLM understands *when* and *how* to use the memory vault.

## 1. The Core Symbiosis Prompt (Base)
This is the essential prompt. It guarantees that the AI acts in symbiosis with `goulm-memory`, reading before acting and continuously updating knowledge as the project evolves.

```text
[MEMORY PROTOCOL]
You have access to a permanent memory vault via `memory_remember` and `memory_recall`.
1. READ FIRST: Before making broad architectural assumptions, configuring environments, or starting a new feature, ALWAYS call `memory_recall` to retrieve existing context.
2. PROACTIVE STORAGE: Call `memory_remember` immediately when we finalize a decision, solve a complex bug, or establish a new standard. 
3. EVOLVE KNOWLEDGE: If you learn that a past decision or fact is outdated, call `memory_remember` with the same key to update/merge the new context.
Use explicit, slug-like keys (e.g., "auth-jwt-strategy", "postgres-connection-bug"). Do not store ephemeral code snippets; store the *logic* and *rationale*.
```

## 2. Modular Add-ons (Build on Demand)
If you have exposed advanced endpoints (like pin, resolve, or suggest), do not overwhelm the LLM. Append **only the specific modules you need** to the Core Prompt above.

### Module A: Pinning (For strict architectural rules)
```text
4. PIN: If a user establishes an unbreakable rule (e.g., "Never use Tailwind"), call `memory_pin` on that key to ensure it is never archived and always prioritized.
```

### Module B: Bug Resolution (For DevOps / QA Agents)
```text
5. RESOLVE: If you successfully fix a bug that was previously documented in the vault, call `memory_resolve` on its key to close the loop and maintain a clean state.
```

### Module C: Suggestions (For Large Codebase Exploration)
```text
6. SUGGEST: When exploring a massive, unfamiliar project, use `memory_suggest` to discover hidden relationships and relevant search terms before making broad recalls.
```

## Prompting Best Practices
1. **Avoid Role-Play Fluff:** Do not waste tokens saying "You are an expert engineer managing a database...". Focus purely on operational *triggers*.
2. **Trigger Words:** Use strong modal verbs (`ALWAYS`, `PROACTIVE`). Modern instruction-tuned models respond exceptionally well to uppercase constraints.
