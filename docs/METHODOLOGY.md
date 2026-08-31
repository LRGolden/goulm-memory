# Methodology & Best Practices

For `goulm-memory` to function as a high-performance brain rather than a "context blender", it is critical to understand that **the library is a mathematical vault, not a magical processor**. 

The quality of the context returned by the hybrid search (`SmartRecall`) is directly proportional to the quality and hygiene of the data ingested via `Remember()`. Follow these 4 guidelines for a professional implementation in your IDE or Agent:

## 1. Semantic Chunking

**Rule:** Do not insert massive raw files as a single capsule.

If you read a 2,000-line `server.go` file and send it as a single `Content`, the generated vector will dilute all ideas. The result will be unrecoverable semantic noise.

* **Incorrect:** `Remember(Content: <2000 lines of text>)`
* **Professional Use:** Your Agent must parse the code and create an independent capsule for each logical block (e.g., function or class). Example: `Remember(Key: "server.go:StartServer", Content: "func StartServer() {...}")`.

## 2. Noise Isolation (Context Preservation)

**Rule:** Keep long-term memory free of repetitive logs, temporary errors, or irrelevant searches.

An AI Agent produces a lot of "temporary garbage" (the output of a failed test, a quick web search). If this enters the vector database, it will ruin the context of future queries.

* **Solution:** Use "Isolated Tabs" architectures in your application, and leverage the `PathScope` or `SessionFiles` field of the library to isolate this noise, or simply **do not persist** temporary data.

## 3. Dense Session Summaries

**Rule:** Do not save the chat history word-for-word in persistent memory.

Conversations can span dozens of turns filled with greetings, buggy code, or conversational text. 

* **Solution:** When a session ends or the context fills up, ask the AI itself to reflect: *"Summarize the key architectural decisions made in this session in 3 bullet points"*. Save **that dense summary** using `Remember()`. 

## 4. Pins for Core Rules

**Rule:** Protect absolute guidelines from temporal decay.

The library uses mathematical time-based decay (`QualityScore`) to gradually forget the irrelevant. However, there are rules that the agent must *never* forget (e.g., *"Always use Go 1.21"*, or *"Do not use global variables"*).

* **Solution:** Use `MemoryStore.Pin(key, priority)`. Capsules with high priority (1-5) bypass the `ArchiveOld` routines and temporal decay, ensuring they are always predictably injected.
