# Integración del Model Context Protocol (MCP)

`goulm-memory` proporciona una implementación nativa y sin dependencias externas del [Model Context Protocol (MCP)](https://modelcontextprotocol.io). Esto permite una integración perfecta con IDEs de IA modernos (como Cursor, Windsurf o Claude Desktop), exponiendo nativamente todas las herramientas de memoria.

## El Servidor `cmd/mcp`

El paquete `cmd/mcp` ejecuta un servidor estricto JSON-RPC 2.0 sobre la entrada/salida estándar (`stdio`). Esto significa que el IDE orquesta la ejecución del proceso localmente sin requerir configuración de red o carga HTTP.

## Configuración en el IDE

Para habilitar `goulm-memory` en tu IDE, apunta la configuración de MCP al binario compilado o ejecútalo directamente vía `go run`.

### Cursor / Windsurf (`mcp.json`)

Añade lo siguiente a la configuración MCP de tu IDE:

```json
{
  "mcpServers": {
    "goulm-memory": {
      "command": "go",
      "args": [
        "run", 
        "./cmd/mcp", 
        "-dir", "/ruta/absoluta/a/tu/espacio/de/trabajo", 
        "-project", "nombre-de-mi-proyecto"
      ]
    }
  }
}
```

### Binario Precompilado

Si prefieres distribuir un único binario sin necesidad de tener Go instalado en la máquina anfitriona:

```bash
go build -o goulm-memory-mcp ./cmd/mcp
```

```json
{
  "mcpServers": {
    "goulm-memory": {
      "command": "/ruta/absoluta/al/goulm-memory-mcp",
      "args": ["-dir", "/ruta/absoluta/a/tu/espacio/de/trabajo"]
    }
  }
}
```

## Herramientas Disponibles

Una vez conectado, el servidor MCP expone automáticamente el `tools.Registry`, inyectando las siguientes herramientas directamente en el contexto de la IA:

*   **`memory_remember`**: Guarda hechos, bugs, decisiones o notas arquitectónicas.
*   **`memory_recall`**: Búsqueda semántica a través de la bóveda usando el pipeline híbrido BM25+VPTree.
*   **`memory_stats`**: Visualiza la salud del repositorio, archivos rotos y enlaces huérfanos.
*   **`memory_forget` / `memory_resolve`**: Gestiona el ciclo de vida de las cápsulas.
*   **`ledger_append`**: Mantiene un historial inmutable (si se usa el hook del Ledger).

## Diagnósticos y Resolución de Problemas

Debido a que MCP usa `stdio` para los mensajes del protocolo, **nunca debes imprimir logs de depuración a `stdout`**. El servidor `cmd/mcp` dirige estrictamente todos los logs y errores estándar hacia `stderr`.

Si tu IDE falla al conectar:
1. Asegúrate de que la ruta `-dir` sea absoluta y tenga permisos de lectura/escritura.
2. Revisa la pestaña Output/Console del MCP en el IDE para ver rastros en `stderr`.
3. El servidor responde al método JSON-RPC `"ping"` para evitar que los clientes agresivos corten la conexión por inactividad.
