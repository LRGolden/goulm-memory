# System Prompts para goulm-memory

Dado que `goulm-memory` es un motor explícito, los agentes no lo usarán de forma automática. Debes inyectar instrucciones en el **System Prompt** de tu agente para que el LLM entienda *cuándo* y *cómo* usar la bóveda de memoria.

## 1. El Prompt Base de Simbiosis
Este es el prompt esencial. Garantiza que la IA actúe en verdadera simbiosis con `goulm-memory`, leyendo antes de actuar y actualizando continuamente el conocimiento a medida que el proyecto evoluciona.

```text
[MEMORY PROTOCOL]
You have access to a permanent memory vault via `memory_remember` and `memory_recall`.
1. READ FIRST: Before making broad architectural assumptions, configuring environments, or starting a new feature, ALWAYS call `memory_recall` to retrieve existing context.
2. PROACTIVE STORAGE: Call `memory_remember` immediately when we finalize a decision, solve a complex bug, or establish a new standard. 
3. EVOLVE KNOWLEDGE: If you learn that a past decision or fact is outdated, call `memory_remember` with the same key to update/merge the new context.
Use explicit, slug-like keys (e.g., "auth-jwt-strategy", "postgres-connection-bug"). Do not store ephemeral code snippets; store the *logic* and *rationale*.
```
*(Nota: Se recomienda mantener estos prompts operativos en inglés, ya que los LLMs responden mejor a instrucciones de function-calling en su idioma nativo).*

## 2. Módulos Avanzados (Armables a Demanda)
Si has expuesto los endpoints avanzados (pin, resolve, suggest), no abrumes al LLM con un prompt gigante. Simplemente **añade los módulos específicos que necesites** al final del Prompt Base.

### Módulo A: Pinning (Fijar reglas estrictas)
```text
4. PIN: If a user establishes an unbreakable rule (e.g., "Never use Tailwind"), call `memory_pin` on that key to ensure it is never archived and always prioritized.
```

### Módulo B: Resolución de Bugs (Para Agentes DevOps/QA)
```text
5. RESOLVE: If you successfully fix a bug that was previously documented in the vault, call `memory_resolve` on its key to close the loop and maintain a clean state.
```

### Módulo C: Sugerencias (Para explorar código inmenso)
```text
6. SUGGEST: When exploring a massive, unfamiliar project, use `memory_suggest` to discover hidden relationships and relevant search terms before making broad recalls.
```

## Mejores Prácticas de Prompting
1. **Cero Role-Play:** No desperdicies tokens diciendo "Eres un ingeniero de software nivel Staff...". Enfócate puramente en *gatillos* operativos.
2. **Palabras de Acción:** Usa verbos en MAYÚSCULAS (`ALWAYS`, `PROACTIVE`). Los modelos modernos (Claude 3.5, GPT-4o) están entrenados para acatar restricciones absolutas marcadas de esta forma.
