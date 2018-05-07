# Import

Import Slack archives into SlackArchive.

# Usage

```
import {xoxb-token} {path-to-archive}
```

# Docker

Copy config.yaml.sample to config.yaml and update it to the correct values. Extract the zip file in the ./data folder and then execute the following command:

```
docker run  --mount type=bind,source=$(pwd)/config.yaml,target="/config/config.yaml" --mount type=bind,source=$(pwd)/data/,target="/data" --network slackarchive dutchcoders/slackarchive-import -- {xoxb-token} {/data/folder}
```
