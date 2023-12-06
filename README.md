# q2admin-cloud
A server for remote management of Quake 2 gameservers using the q2admin game mod. The game mod will keep an open TCP connection to this server process and feed it information. This server can then take action based on that information. It maintains global and server-specific ban/mute lists, captures statistics, and allows for general server management.

## Components
- Main server process
- Web interface for management
- CLI management app

## Authentication and authorization
Clients and servers mutually authenticate using asymmetric encryption keys. The server and client exchange public keys out-of-band ahead of making a connection, while setting up the connection in the web interface.

## Encryption
The TCP connection between the server and clent can be encrypted via a flag in the client's q2admin config. If configured, the packets are encrypted using AES-128-CBC. Encryption keys are randomly generated and rotated periodically. Disabling encryption can save processor overhead, but should really only be done where client and server are on the same machine. Server can support both encrypted and non-encrypted clients simultaneously.

## Configuration
The main config file is named `config/config` but can be specified at the runtime via the `--config` flag. All configs are in text-based protocol buffer format. Example:
```
# proto-file: proto/config.proto
# proto-message: Config

debug_mode: true
address: "0.0.0.0"
port: 9988
database: "database/q2a.sqlite"
private_key: "crypto/private.pem"
api_enabled: true
api_address: "0.0.0.0"
api_port: 8087
maintenance_time: 60
client_file: "config/clients"
client_directory: "clients"
user_file: "config/users"
access_file: "config/access"
auth_file: "config/oauth"
web_root: "api/website"
log_file: "server.log"
```
