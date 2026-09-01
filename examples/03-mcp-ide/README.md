# MCP IDE Integration Example

This folder contains a sample `mcp.json` file to instantly connect `goulm-memory` to an AI-powered IDE (like Cursor, Windsurf, or Claude Desktop) using the Model Context Protocol (MCP).

## How to use it in Cursor
1. Copy the contents of `mcp.json`.
2. In Cursor, open Settings -> Features -> MCP.
3. Click "Add New MCP Server".
4. Name it `goulm-memory` and select the type `command`.
5. Paste the command and arguments exactly as shown in the JSON file. Ensure you update the absolute path to the `cmd/mcp` directory if you are not running it from the project root.
6. Refresh the MCP server list. You will instantly see all 13 tools (e.g., `memory_remember`, `memory_recall`) become available to the IDE agent.
