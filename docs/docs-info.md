While developing the connector, please fill out this form. This information is needed to write docs and to help other users set up the connector.

## Connector capabilities

1. What resources does the connector sync?

    -Users
    -Groups
    -Roles

2. Can the connector provision any resources? If so, which ones?

    -Groups

## Connector credentials

1. What credentials or information are needed to set up the connector? (For example, API key, client ID and secret, domain, etc.)

    -API Key
    -Domain

2. For each item in the list above:

	* How does a user create or look up that credential or info? Please include links to (non-gated) documentation, screenshots (of the UI or of gated docs), or a video of the process.

	Log into Twingate, go to Settings -> General -> API -> Generate Token (This will be the API Key)

    To find your domain, copy your URL and take the first string before a ".", for example
    https://c11.twingate.com/settings/api -> your domain is c11

	* Does the credential need any specific scopes or permissions? If so, list them here.
	
    The API Key should be able to Read, Write and Provision

	* If applicable: Is the list of scopes or permissions different to sync (read) versus provision (read-write)? If so, list the difference here.
	
	Yes. For read-only operations (sync), the API key only needs `Read` permissions. For provisioning, both `Read`, `Write` and `Provision` permissions are required.
	
	* What level of access or permissions does the user need in order to create the credentials? (For example, must be a super administrator, must have access to the admin console, etc.)
	
	The user must be an Administrator in the Twingate account to be able to access the API settings page and generate tokens.
	
