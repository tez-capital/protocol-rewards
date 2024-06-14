# ogun

config.hjson
```hjson
{
   environment: development
   listen: [
      127.0.0.1:3000
   ]
   subsystems: {
      tezos: {
         providers: [
            https://rpc.tzkt.io/mainnet/
         ]
         number_of_active_providers: 2
      }
   }
   database: {
      host: 127.0.0.1
      port: 5432
      user: tezwatch1
      password: tezwatch1
      database: tezwatch1
   }
   batch_size: 5
   allow_manual_cycle_fetching: true
}
```