# Q2Admind
This is the remote-admin server that works with the q2admin game mod. The game mod will keep an open TCP connection to this server process and feed it information. This server can then take action based on that information. For example, this server maintains a ban/mute list. When players connect to game servers, this server is notified and can send back a command to kick/ban the player if a ban applies to them.

## Components
- Main server process (for game servers to connect to)
- Web interface (for server owners to add and manage their servers)

## Authentication and authorization
Game servers initiate connections to this server which authenticates them using public/private keys. The game server owner must download our public key onto their server and upload their game server's public key into the web interface.

## Encryption
The TCP connection between the game server mod and this server can be encrypted via a flag in the config file. If configured, the packets between the game server and this system are encrypted using AES-256-CBC. Encryption keys are randomly generated and rotated periodically. Disabling encryption can save processor overhead, but should really only be done where game servers and management servers are on the same machine.

## Configuration
The default config file is `q2a.json` in the same directory as the `q2admind` binary. Example:
```
{
    "address": "0.0.0.0",
    "port": 9988,
    "database": "server.sqlite",
    "privatekey": "private-1628817495.pem",
    "apiport": 8087,
    "debug": 0,
    "enableapi": 0
}
```
