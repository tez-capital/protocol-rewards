# ogun

config.hjson
```hjson
{
    listen: 127.0.0.1:3000
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
}
```

.env
```
LOG_LEVEL=debug

```

LOG_LEVEL accepted values are debug, info, warn, error. Defaults to info level.

U can define env variables in the .env file or in your environment directly as you choose. If you forgot to define your env variable they will be assigned the default values.