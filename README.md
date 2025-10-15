# iRODS MCP Server

The iRODS MCP Server provides access to data in iRODS. This project contains only the public, iRODS-related portions of the code for the AI Verde Data Store MCP Server.

## Execution Modes

The iRODS MCP Server can run on an MCP Client machine using Docker in STDIO mode.

The iRODS MCP Server can also run on a dedicated server for multiple client users. In this case, the server supports both `HTTP/SSE` and `Streamable-HTTP`.

## Run in an MCP Client Machine with Docker (with Claude Desktop)

Edit the `~/.config/Claude/claude_desktop_config.json` file.

After editing, restart Claude Desktop to apply the changes.

### a. Using Anonymous Access

This configuration allows access only to public data located at `/<zone>/home/shared` or `/<zone>/home/public`.

```json
{
    "mcpServers": {
        "irods": {
            "command": "docker",
            "args": [
                "run",
                "-i",
                "--rm",
                "cyverse/irods-mcp-server"
            ]
        }
    }
}
```

### b. Using iRODS Account

This configuration allows access to your iRODS home directory (`/<zone>/home/<username>`) plus public data. Replace `irods_username` and `irods_password` with your actual credentials.

```json
{
    "mcpServers": {
        "irods": {
            "command": "docker",
            "args": [
                "run",
                "-i",
                "--rm",
                "-e",
                "USERNAME=irods_username",
                "-e",
                "PASSWORD=irods_password",
                "cyverse/irods-mcp-server"
            ]
        }
    }
}
```

## Run as a Server

Create a file named `config.yaml` and add the following:
```yaml
remote: true
service_url: http://:8080
background: true
debug: true
log_path: ./irods-mcp-server.log
```

With this configuration, the server:  
- Listens for incoming connections on **port 8080**  
- Supports both **HTTP/SSE** and **Streamable-HTTP** requests  
- Saves all logs (including debug info) to a file named `irods-mcp-server.log`  

Run the iRODS MCP Server executable using the command:
```bash
irods-mcp-server -c config.yaml
```

Once started, the server provides two endpoints:

- Endpoint URL for HTTP/SSE: `http://localhost:8080/sse`
- Endpoint URL for Streamable-HTTP service: `http://localhost:8080/mcp`

## Setup VS Code for Anonymous Access

Edit the `~/.config/Code/User/mcp.json` file.

This configuration allows access only to public data located at `/<zone>/home/shared` or `/<zone>/home/public`.

Replace the URL `http://localhost:8080/mcp` with the actual one where you are running the iRODS MCP Server.

```json
{
    "servers": {
        "irods": {
            "type": "http",
            "url": "http://localhost:8080/mcp"
        }
    }
}
```

## Setup VS Code for iRODS Account

Create a key from your iRODS account.

Use your CyVerse username (e.g., `foo`) and password (e.g., `mypassword`) separated by a colon (`:`) in the command below:
```bash
echo -n "foo:mypassword" | base64
```

The created key will be displayed in the Terminal.

Edit the `~/.config/Code/User/mcp.json` file.

This configuration allows access to your iRODS home directory (`/<zone>/home/<username>`) plus public data.

Replace the URL `http://localhost:8080/mcp` with the actual one where you are running the iRODS MCP Server.

Replace the key `YOUR_BASE64_KEY` with the actual one created from your iRODS credentials. Your key must come after `Basic ` (including the space).

```json
{
    "servers": {
        "irods": {
            "type": "http",
            "url": "http://localhost:8080/mcp",
            "headers": {
				"Authorization": "Basic YOUR_BASE64_KEY"
			}
        }
    }
}
```