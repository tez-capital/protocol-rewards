# Protocol Rewards

config.hjson
```hjson
{
   providers: [
      https://eu.rpc.tez.capital/
      https://rpc.tzkt.io/mainnet/
   ]
   tzkt_providers: [
       https://api.tzkt.io/
   ]
   database: {
      host: 127.0.0.1
      port: 5432
      user: protocol_rewards
      password: protocol_rewards
      database: protocol_rewards
   }
   storage: {
      mode: rolling
      stored_cycles: 20
   }
   discord_notificator: {
      webhook_url: url
      webhook_id: id
      webhook_token: token
   }
   // optional subset if wanted, if not just delete it or keep it empty
   delegates: [
      tz1P6WKJu2rcbxKiKRZHKQKmKrpC9TfW1AwM
      tz1LVqmufjrmV67vNmZWXRDPMwSCh7mLBnS3
      tz1WzjeZrQm2JJT43rk7USfmnSQ2nLSebtta
   ]
}
```

.env
```
LOG_LEVEL=debug
LISTEN=127.0.0.1:3000
PRIVATE_LISTEN=127.0.0.1:4000

```

LOG_LEVEL accepted values are debug, info, warn, error. Defaults to info level.

U can define env variables in the .env file or in your environment directly as you choose. If you forgot to define your env variable they will be assigned the default values.

testing command flags
```
go run main.go -log debug -test tz1gXWW1q8NcXtVy2oVVcc2s4XKNzv9CryWd:745
```

### Credits

**Powered by [TzKT API](https://api.tzkt.io/)** - `protocol-rewards` use TZKT api to fetch unstake requests.