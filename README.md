# Audiobookshelf MCP Server

A Model Context Protocol (MCP) server that provides tools to interact with your [Audiobookshelf](https://www.audiobookshelf.org/) instance. Access your libraries, audiobooks, podcasts, authors, collections, and playlists through AI assistants that support MCP.

## Disclaimer

**USE AT YOUR OWN RISK:** This software is provided "as is" without warranty of any kind. The author is not responsible for any misuse, accidents, data loss, or any other issues that may arise from using this software. Users are solely responsible for ensuring proper configuration and usage of this tool with their Audiobookshelf instance.

---

## Features

- List and retrieve libraries with optional sub-resources (items, authors)
- Get individual items (audiobooks or podcasts)
- Browse authors and their works
- Access collections and playlists
- Retrieve user information

## Installation

### Prerequisites

- An Audiobookshelf instance with API access
- An API token from your Audiobookshelf instance

### Build from Source

Build from source or use Docker Compose below to install the server today.

**Prerequisites:**
- Go 1.25.2 or later

```bash
git clone https://github.com/schmidt-software/audiobookshelf-mcp.git
cd audiobookshelf-mcp
go build -o abs-mcp .
```

### Docker Compose

The server can run as a container exposing the MCP streamable HTTP transport (`http://localhost:8080/mcp`).

```bash
cp .env.example .env   # set ABS_BASE_URL and ABS_API_KEY
docker compose up -d --build
```

Transport settings (optional environment variables): `MCP_TRANSPORT` (`stdio` default outside Docker, `http` in the image), `MCP_ADDR` (default `:8080`), `MCP_ENDPOINT` (default `/mcp`), and `MCP_PORT` for the published host port in compose.

Example client configuration:

```json
{ "mcpServers": { "audiobookshelf": { "url": "http://localhost:8080/mcp" } } }
```

### Pre-built Releases (once published)

This repository does not currently publish release downloads. Once releases are available, download the appropriate archive for your platform from the [latest release](https://github.com/schmidt-software/audiobookshelf-mcp/releases/latest). Archive names may vary by release.

**Installation steps for future release archives:**

1. Download the appropriate archive for your platform from the releases page
2. Extract the archive, replacing the filename with the archive you downloaded:
   ```bash
   tar -xzf <archive-name>.tar.gz
   ```
3. Move the binary to a location in your PATH (optional but recommended):
   ```bash
   # macOS/Linux
   sudo mv abs-mcp /usr/local/bin/

   # Or to a user directory
   mv abs-mcp ~/.local/bin/
   ```
4. Make it executable (macOS/Linux):
   ```bash
   chmod +x /usr/local/bin/abs-mcp
   ```

## Configuration

The MCP server requires two pieces of configuration:

1. **ABS_BASE_URL** - The server URL of your Audiobookshelf instance, without `/api` (e.g., `https://abs.example.com`, or `https://abs.example.com/abs` when served under a reverse-proxy base path)
2. **ABS_API_KEY** - Your Audiobookshelf API token

### Getting Your API Token

1. Log into your Audiobookshelf instance
2. Go to Settings → Users → Your User
3. Click "Generate API Token" or copy your existing token

## Usage

### Environment Variables

You can set the configuration using environment variables:

```bash
export ABS_BASE_URL="https://abs.example.com"  # no /api suffix
export ABS_API_KEY="your-api-token-here"
```

Alternatively, you can pass these as parameters when calling tools (see Tool Parameters below).

### Setting Up with Witsy

[Witsy](https://github.com/nbonamy/witsy) is a desktop AI assistant that supports MCP servers. Here's how to set it up:

1. Open Witsy and go to Settings (⚙️ icon)
2. Navigate to the MCP section
3. Add a new server with the following configuration:

![Witsy MCP Configuration](_res/witsy.png)

**Configuration details:**
- **Type**: `stdio`
- **Label**: `abs-mcp` (or any name you prefer)
- **Command**: `/path/to/abs-mcp` (use the "Pick" button to select your compiled binary)
- **Arguments**: (leave empty)
- **Working Directory**: Any directory (e.g., `/Users/yourname/Downloads`)
- **Environment Variables**:
  - `ABS_BASE_URL` = `https://example.library.abs` (your Audiobookshelf URL, without `/api`)
  - `ABS_API_KEY` = `IM_A_LONG_STRING` (your API token)

4. Click "Save" to add the server
5. The server will now be available in your Witsy conversations!

### Setting Up with Claude Desktop

Add this to your Claude Desktop configuration file:

**macOS**: `~/Library/Application Support/Claude/claude_desktop_config.json`
**Windows**: `%APPDATA%\Claude\claude_desktop_config.json`

```json
{
  "mcpServers": {
    "audiobookshelf": {
      "command": "/path/to/abs-mcp",
      "env": {
        "ABS_BASE_URL": "https://abs.example.com",
        "ABS_API_KEY": "your-api-token-here"
      }
    }
  }
}
```

## Available Tools

### Libraries

- **libraries** - List all libraries
- **library** - Get a single library by ID, or fetch specific library sub-resources:
  - `items=true` - Get all items in the library
  - `authors=true` - Get all authors in the library
  - `series=true` - Get all series in the library
  - `collections=true` - Get all collections in the library
  - `playlists=true` - Get all playlists in the library
  - `personalized=true` - Get personalized view for the library
  - `filterdata=true` - Get filter data for the library
  - `stats=true` - Get library statistics
  - `search=true` + `query=<text>` - Search library items
  - Optional with search: `limit=<number>` - Limit the number of search results
  - `episode-downloads=true` - Get episode downloads for the library
  - `recent-episodes=true` - Get recent episodes for the library
- **create_library** - Create a new library
  - Required: `name`, `folders` (comma-separated paths), `media_type` (book or podcast)
  - Optional: `icon`, `provider`

### Items

- **item** - Get a single item (audiobook or podcast) by ID, or fetch specific item sub-resources:
  - `cover=true` - Get the cover image for the item as MCP image content
  - `tone-object=true` - Get the tone object for the item

### Authors

- **author** - Get a single author by ID
- **author_image** - Get an author's image by ID as MCP image content

### Collections

- **collections** - List all collections
- **collection** - Get a single collection by ID
- **create_collection** - Create a new collection
  - Required: `library_id`, `name`
  - Optional: `description`
- **add_to_collection** - Add a book to an existing collection
  - Required: `collection_id`, `book_id`

### Playlists

- **playlists** - List all playlists
- **playlist** - Get a single playlist by ID
- **create_playlist** - Create a new playlist
  - Required: `library_id`, `name`
  - Optional: `description`
- **add_to_playlist** - Add an item to an existing playlist
  - Required: `playlist_id`, `item_id`
  - Optional: `episode_id` (for podcast episodes)

### User

- **authorize** - Get authorized user and server information
- **me** - Get authenticated user information, or fetch specific user sub-resources:
  - `listening-sessions=true` - Get listening sessions for the user
  - `listening-stats=true` - Get listening statistics for the user
  - `items-in-progress=true` - Get items currently in progress for the user
  - `progress_item_id=<id>` - Get progress for a specific library item
  - `progress_item_id=<id>` + `progress_episode_id=<id>` - Get progress for a specific episode

### Sessions

- **sessions** - List all playback sessions
- **session** - Get a single playback session by ID

### Podcasts

- **podcasts** - List podcast library items, fetch feed metadata, or parse OPML text:
  - `library_id=<id>` - Required for listing podcasts; lists items from a podcast library
  - `feed=true`, `rss_feed=<url>` - Fetch podcast RSS feed metadata
  - `opml=true`, `opml_text=<xml>` - Parse OPML text for feed URLs
- **podcast** - Get a podcast library item by ID, or fetch podcast sub-resources:
  - Required: `podcast_id` (the podcast library item ID)
  - `downloads=true` - Get downloads for the podcast
  - `search-episode=true` + `title=<text>` - Search for episodes in the podcast
  - `episode_id=<id>` - Get a specific episode by ID
- **check_podcast_episodes** - Check for new episodes for a podcast
  - Required: `podcast_id` (the podcast library item ID)
  - Optional: `limit` (maximum number of new episodes to download; Audiobookshelf defaults to 3)

### Progress Tracking

- **update_progress** - Update listening progress for a media item
  - Required: `item_id`, `progress` (in seconds)
  - Optional: `duration` (in seconds), `is_finished` (boolean; sends true or false when provided), `episode_id` (for podcasts)

### Backups

- **create_backup** - Create a server backup

## Tool Parameters

All tools accept optional `base_url` and `token` parameters that override the environment variables. Use the Audiobookshelf server URL without `/api`; a reverse-proxy base path such as `/abs` is supported:

```json
{
  "base_url": "https://abs.example.com",
  "token": "your-api-token-here"
}
```

This is useful if you need to access multiple Audiobookshelf instances or prefer not to use environment variables.

## Example Queries

Once configured, you can ask your AI assistant questions like:

**Reading and browsing:**
- "Show me all my audiobook libraries"
- "What items are in my Fiction library?"
- "Get details about the author with ID abc123"
- "List all my playlists"
- "What's in my Currently Reading collection?"

**Creating and managing:**
- "Create a new collection called 'Summer Reads' in my library"
- "Add this book to my 'Currently Reading' collection"
- "Create a new playlist for my favorite sci-fi audiobooks"
- "Check for new podcast episodes"
- "Update my progress to 45 minutes on this audiobook"
- "Create a backup of my server"

The AI assistant will use the appropriate MCP tools to fetch information and manage your Audiobookshelf instance.

## Development

### Project Structure

- `main.go` - Main server implementation
- Helper functions for API authentication and request handling
- MCP tool definitions and handlers

### Adding New Tools

To add a new tool:

1. Define the tool options using `mcp.NewTool()`
2. Add authentication parameters with `withABSAuth()`
3. Register the tool with `s.AddTool()`
4. Use helper functions like `createSimpleGETHandler()` or `createGETByIDHandler()`

## License

GNU General Public

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.

## Support

For issues related to:
- This MCP server: Open an issue in this repository
- Audiobookshelf: Visit [audiobookshelf.org](https://www.audiobookshelf.org/)
- MCP Protocol: See [modelcontextprotocol.io](https://modelcontextprotocol.io/)
