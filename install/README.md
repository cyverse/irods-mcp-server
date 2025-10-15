# Setup iRODS MCP Server systemd service

Use `Makefile` in install package. 

```bash
sudo make install
```

Enable the service.
```bash
sudo systemctl enable irods-mcp-server.service
```

Start the service.
```bash
sudo service irods-mcp-server start
```

Check the service status.
```bash
sudo service irods-mcp-server status
```