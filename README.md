# Remote Risk Engine Demo

This repository shows how to write a **Remote Risk Engine** plugin
for North Pole Security's [Workshop](https://northpole.security).

This example plugin evaluates binaries based on their App Store presence. It verifies code signing information to confirm App Store origin and queries the iTunes Search API to determine the application's release date. The plugin implements a simple risk policy: applications released on the App Store less than 30 days ago are denied, while those with longer market presence are allowed.

> [!WARNING]
> This code is intended only for demo purposes and should not be
considered production ready.
>
> The iTunes Search API is limited to 20 queries per minute and
> this server uses hard coded secrets.

## Prerequisites

Make sure you have Workshop running. For this example we'll assume Workshop is listening on `localhost:8080`.

You'll need to create an API key with read-write access. Workshop API keys begin with `npsws_sk_`.

## Build & Run

```sh
$ go build -o plugin-server ./cmd/server.go
$ ./plugin-server
```

The plugin server should now be listening at localhost on port 8888.

## Enable the Plugin

First, configure Workshop to use the plugin using the `UpdateSettings` API method using the JSON payload below. We'll assume that Workshop is running in Docker, so Workshop will need to point to `http://host.docker.internal:8888` in order to access the plugin server:

```json
{
  "enabled": true,
  "remotePlugins": [
    {
      "enabled": true,
      "name": "demo",
      "version": "0.0.1",
      "url": "http://host.docker.internal:8888",
      "headers": [{"key": "X-API-Key", "value": "sekrit"}],
      "ttl": "120.0s"
    }
  ]
}
```

You can send an API request using the `gprcurl` command like this:

```sh
$ grpcurl -plaintext \
  -H "Authorization: $WORKSHOP_API_KEY" \
  -d '{"riskEngineSettings":{"enabled":true,"remotePlugins":[{"enabled":true,"name":"demo-plugin","version":"0.0.1","url":"http://host.docker.internal:8888","headers":[{"key":"X-API-Key","value":"sekrit"}],"ttl":"120.0s"}]}}' \
  localhost:8080 workshop.v1.WorkshopService/UpdateSettings
```

Check that the settings were applied using the `GetSettings` method:

```sh
$ grpcurl -plaintext \
  -H "Authorization: $WORKSHOP_API_KEY" \
  localhost:8080 workshop.v1.WorkshopService/GetSettings

{
  ...
  "riskEngineSettings": {
    "enabled": true,
    "pluginTimeout": "120s",
    "localPlugins": {
      ...
    },
    "remotePlugins": [
      {
        "enabled": true,
        "name": "demo-plugin",
        "version": "0.0.1",
        "uuid": "a3edf34c-5825-46cd-b121-0b2681c4bd44",
        "url": "http://host.docker.internal:8888",
        "headers": [
          {
            "key": "X-API-Key",
            "value": "sekrit"
          }
        ],
        "ttl": "120s"
      }
    ]
  }
}
```

## Test the Plugin

You can check the Remote Risk Engine plugin is working properly by using the `CheckBlockable` API method to check a binary. We've picked an arbitrary SHA256 as an identifier for this example, but you could get one yourself using `santactl fileinfo <path-to-binary>` if you wanted.

```sh
$ grpcurl \
  -plaintext -H "Authorization: $WORKSHOP_API_KEY" \
  -d '{"blockable": {"sha256": "4e4eea34dc9d936ba7d60f8814dc1d0c87d48c88d68bbf14cde97d8a33663842"}}' \
  localhost:8080 workshop.v1.WorkshopService/CheckBlockable

{
  "results": [
    {
      "timestamp": "2025-03-05T18:30:06.041106709Z",
      "goodUntil": "3000-12-25T00:00:00Z",
      "txId": "050bc98c-189b-4da6-9665-871955f769dd",
      "allowed": true,
      "decision": "DECISION_ALLOW",
      "pluginName": "Blockable Rules (1.0.0)",
      "pluginUuid": "51d20397-d4f2-4bc8-915b-137dcb841548",
      "explanation": "No rules matched"
    },
    {
      "timestamp": "2025-03-05T18:30:06.041106709Z",
      "goodUntil": "1970-01-01T00:00:00Z",
      "txId": "050bc98c-189b-4da6-9665-871955f769dd",
      "allowed": true,
      "decision": "DECISION_ALLOW",
      "pluginUuid": "271b581e-498c-4ef0-95f2-57cdc6330e22",
      "explanation": "Binary is not not from the app store"
    },
    {
      "timestamp": "2025-03-05T18:30:06.041106709Z",
      "goodUntil": "2025-03-05T18:31:06.040980876Z",
      "txId": "050bc98c-189b-4da6-9665-871955f769dd",
      "allowed": true,
      "decision": "DECISION_ALLOW",
      "pluginName": "VirusTotal (1.0.0)",
      "pluginUuid": "8f239575-826e-4909-9489-a6d36d663b7a",
      "explanation": "File is not known malicious at this time"
    }
  ]
}
```

When Workshop calls the plugin, it should log its decisions to stderr:

```sh
$ ./plugin-server
2025/03/04 20:00:53 Starting HTTP server on port 8888...
2025/03/04 20:01:09 Binary:  Things3
2025/03/04 20:01:09 Decision: DECISION_ALLOW
2025/03/04 20:01:09 Team ID:  JLMPQHK86H
2025/03/04 20:01:09 App Store URL:  https://apps.apple.com/us/app/things-3/id904280696?mt=12&uo=4
2025/03/04 20:01:09 Good until:  3000-12-25 00:00:00 +0000 UTC
```

## Workflow Diagram

```mermaid
flowchart TD
A[Receives an Authz request for a Binary]
B{Is this binary signed by the App Store?}
C{Has this binary only been on
  the App Store
  for less than 30 days?}
X[Allow Binary]
Y[Deny Binary]

A --> B
B --> |No|X
B --> |Yes|C
C --> |No|X
C --> |Yes|Y
```

## Interaction Diagram

```mermaid
sequenceDiagram
Workshop ->> Plugin: Make an AuthzRequest for a binary
Plugin ->> iTunes Search API: Search for binary on App Store
Plugin ->> Plugin: Evaluate binary against App Store information
Plugin -->> Workshop: Return policy decision
```
