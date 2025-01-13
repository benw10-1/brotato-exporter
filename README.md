# Brotato Exporter Mod

Mod which exports game-state data via websocket connection.

## Features
  - Configurable websocket which sends messages on changes to the game state

### Planned
  - CI
  - Workshop
  - Historical data storage and retrieval
  - UI for viewing current run state as well as historical state
  - Actual tests
  - Config in-game UI
  - Weapon data

## Setup

### Server setup

1. Install [Docker](https://docs.docker.com/engine/install/).
2. Pull the [image from Dockerhub](https://hub.docker.com/repository/docker/benwirth10/brotato-exporter/general).
3. Run the `mod-user-create.sh` script to interactively setup the mod config. Do this before starting the server.
4. Run the `run-server.sh` script to start the server on port 8081.

You do not have to use the `run-server.sh` script if you want to serve on other ports. See [default.yml](./default.yml) for config options.

Running locally use the same `mod-user-create.sh` script, but run the compose instead.

### Client setup

1. Subscribe to the mod [on Steam](https://steamcommunity.com/sharedfiles/filedetails/?id=3406507312)
2. Run `mod-user-create.sh` and either copy the `user-mod.zip` or `connect-config.json`.
3. Navigate to the workshop folder located usually at `%steamapps%/workshop/content/1942280/3406507312` (ex. `/d/Steam/steamapps/workshop/content/1942280/3406507312`)
4. If you copied `user-mod.zip` just replace the zip file in the `.../1942280/340650731` folder with the `user-mod.zip` folder. If you copied the `connect-config.json` file instead, you need to edit the zip file and place it in the `mods-unpacked/benw10-BrotatoExporter` folder.

If the user was created to point at the right address you should be able to just run the game and have it sending data.

### Dev setup

See the [modding guide](https://steamcommunity.com/sharedfiles/filedetails/?id=2931079751) for information on getting the Godot environment setup.

Aside from that, follow server setup and create a user. After doing this copy the resulting `connect-config.json` to your mod folder.