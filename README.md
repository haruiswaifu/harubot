# Harubot
To run this on your own Twitch account, change the values in `env.json` and `secrets.json` to values that correspond to your account.

You'll need to set up Twitch API secrets, both an Oauth token for IRC and a user token with `user:read:subscriptions` permissions, which is probably easiest to do through twitchtokengenerator.com.

Most of the values in `env.json` are required, but `self-displayname` can be empty if you don't have one.

### Building
Once you've set these values, you can run the bot with:
`make run_docker`