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

- A Workshop instance
- Workshop Admin permissions

For this example we'll assume that your plugin is reachable at
`plugin.example.com:8888`

## Build & Run

```sh
$ go build -o plugin-server ./cmd/server.go
$ ./plugin-server
```

The plugin server should now be listening on port 8888.

## Enable the Plugin

### Using the UI

If you're using the UI, you can simply browse to the `/settings` page and click on Remote Risk Engine plugins tab and fill in the form.

### Via the API

First, configure Workshop to use the plugin using the [`UpdateRiskEngineSettings` method](https://buf.build/northpolesec/workshop-api/docs/main:workshop.v1#workshop.v1.WorkshopService.UpdateRiskEngineSettings) using the JSON payload below changing `plugin.example.com` to the address you're plugin is running at. We'll assume that Workshop is running in
Docker, so Workshop will need to point to `https://plugin.example.com:8888` in
order to access the plugin server:

```json
{
  "enabled": true,
  // ... SNIPPED
  "remotePlugins": [
    {
      "enabled": true,
      "name": "demo",
      "version": "0.0.1",
      "url": "https://plugin.example.com:8888",
      "headers": [{"key": "X-API-Key", "value": "sekrit"}],
      "ttl": "120.0s"
    }
  ]
}
```

Check that the settings were applied using the [`GetRiskEngineSettings` method](https://buf.build/northpolesec/workshop-api/docs/main:workshop.v1#workshop.v1.WorkshopService.UpdateRiskEngineSettings):

```shell
$ grpcurl \
  -H "Authorization: $WORKSHOP_API_KEY" \
  nps.workshop.cloud:443 workshop.v1.WorkshopService/GetRiskEngineSettings
```

You should see your plugin matching:

```json
{
  "riskEngineSettings": {
    "enabled": true,
    // SNIPPED.
    "remote



```


## Test the Plugin

### Testing via the UI

In the UI go to the Risk Engine card on the Settings page and drag in an
application. You should see your remote plugin being called in the list of Risk
Engine Plugins.

TODO put the image here.


### Testing via the API

You can check the Remote Risk Engine plugin is working properly in Workshop by
calling the [`CheckBlockable` method](https://buf.build/northpolesec/workshop-api/docs/main:workshop.v1#workshop.v1.WorkshopService.CheckBlockable) to evaluate a binary. 

The `CheckBlockable` method will fill in other details for the blockable field
from Workshop's database if only the SHA256 is filled in.

In this example we've picked an arbitrary SHA256 as an identifier for this
example, but you could get one yourself using `santactl fileinfo
<path-to-binary>` if you wanted.

```sh
$ grpcurl \
  -plaintext -H "Authorization: $WORKSHOP_API_KEY" \
  -d '{"blockable": {"sha256": "4e4eea34dc9d936ba7d60f8814dc1d0c87d48c88d68bbf14cde97d8a33663842"}}' \
  nps.workshop.cloud:443 workshop.v1.WorkshopService/CheckBlockable

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

When Workshop calls the plugin, you'll see its logs on stderr:

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
Workshop ->> Plugin: Makes an PluginAuthzRequest for a binary
Plugin ->> iTunes Search API: Search for binary on App Store
Plugin ->> Plugin: Evaluate binary against App Store information
Plugin -->> Workshop: Returns a PluginAuthzResponse containing a policy decision
```

## Writing Your Own Remote Risk Engine Plugin

Documentation can be found at [https://docs.workshop.cloud/risk-engine](https://docs.workshop.cloud/risk-engine#writing-your-own-remote-risk-engine-plugins)
