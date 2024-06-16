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