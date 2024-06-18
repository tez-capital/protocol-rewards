# ogun

config.hjson
```hjson
{
   providers: [
      https://eu.rpc.tez.capital/
      https://rpc.tzkt.io/mainnet/
   ]
   database: {
      host: 127.0.0.1
      port: 5432
      user: ogun
      password: ogun
      database: ogun
   }
   storage: {
      mode: rolling
      stored_cycles: 20
   }
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