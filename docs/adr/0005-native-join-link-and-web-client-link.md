# Native Join Link and Web Client Link

We kept the existing `ts3server://` deep link as the primary join action and added a second, optional link to a web-hosted TeamSpeak client configured through `web_client.url`, opened in a new tab. The native client stays primary because only it offers the full voice feature set; the web client exists as a fallback for visitors who do not have the client installed, so it must never replace the native path. The web client URL is passed through verbatim rather than assembled from the configured server host and port, because web client deep-link formats differ across TeamSpeak versions and self-hosted deployments and TSPeek should not guess a third party's URL contract; the value is validated at startup as an absolute http(s) URL so an arbitrary scheme cannot reach an `href`, and an empty value hides the button instead of falling back to the official site.

## Considered Options

- **Replace the native link with the web client.** Rejected: visitors who do have the native client would lose voice quality and features they already have.
- **Build the web client URL from the configured host and port.** Rejected: there is no stable cross-version parameter format, so any guess would break silently on some deployments.
- **Fall back to `https://web.teamspeak.com` when unset.** Rejected: it makes a deployment decision the operator never asked for, and can send visitors somewhere the operator does not control.
