# Metodología de Uso Correcto

Para que `goulm-memory` funcione como un cerebro de alto rendimiento y no como una "licuadora de contexto", es crítico entender que **la librería es una bóveda matemática, no un procesador mágico**. 

La calidad del contexto devuelto por la búsqueda híbrida (`SmartRecall`) es directamente proporcional a la calidad e higiene de los datos introducidos mediante `Remember()`. Sigue estas 4 pautas para una implementación profesional en tu IDE o Agente:

## 1. Chunking Semántico (Fragmentación)

**Regla:** No insertes archivos masivos crudos como una sola cápsula.

Si envías un archivo `server.go` de 2,000 líneas en un solo `Content`, el vector generado diluirá todas las ideas. El resultado será ruido semántico irrecuperable.

* **Uso incorrecto:** `Remember(Content: <2000 líneas de texto>)`
* **Uso profesional:** Tu Agente debe parsear el código y crear una cápsula independiente por cada bloque lógico (ej. función o clase). Ejemplo: `Remember(Key: "server.go:StartServer", Content: "func StartServer() {...}")`.

## 2. Aislamiento del Ruido (Context Preservation)

**Regla:** Mantén la memoria a largo plazo libre de logs repetitivos, errores temporales o búsquedas irrelevantes.

Un Agente de IA produce mucha "basura temporal" (el output de un test fallido, una búsqueda rápida en la web). Si esto entra en la base de vectores, arruinará el contexto de futuras consultas.

* **Solución:** Utiliza arquitecturas de "Pestañas Aisladas" en tu aplicación, y aprovecha el campo `PathScope` o `SessionFiles` de la librería para aislar este ruido, o directamente **no persistas** estos datos temporales.

## 3. Resúmenes Densos de Sesión

**Regla:** No guardes el historial de chat palabra por palabra en la memoria persistente.

Las conversaciones pueden extenderse por docenas de turnos llenos de saludos, código con bugs o texto conversacional. 

* **Solución:** Cuando una sesión termine o el contexto se llene, pide a la propia IA que reflexione: *"Resume las decisiones arquitectónicas clave tomadas en esta sesión en 3 viñetas"*. Guarda **ese resumen denso** usando `Remember()`. 

## 4. Pines para Reglas Maestras (Core Rules)

**Regla:** Protege las directrices absolutas del decaimiento temporal.

La librería utiliza un decaimiento matemático basado en el tiempo (`QualityScore`) para olvidar poco a poco lo irrelevante. Sin embargo, hay reglas que el agente *jamás* debe olvidar (ej. *"Usa siempre Go 1.21"*, o *"No uses variables globales"*).

* **Solución:** Usa `MemoryStore.Pin(key, priority)`. Las cápsulas con prioridad alta (1-5) evaden las rutinas de `ArchiveOld` y el decaimiento temporal, asegurando que se inyecten siempre de forma predecible.
