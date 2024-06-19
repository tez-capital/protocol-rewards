- [ ] Core features
  - [ ] over staking (waiting for NL engineers)

- [ ] tests (test all this works)
  - [ ] test private server
  - [ ] test public server
  - [ ] validate data approximately
  - [ ] docker compose and containers

- [ ] test suite
  - [ ] add test more precise test for engine based on test data from test/data/745.zip
  
- [ ] deploy
  - [ ] prepare nginx config (I'll do this)

- [x] found bugs
  - [x] tz1gXWW1q8NcXtVy2oVVcc2s4XKNzv9CryWd - 746 - minimum not found
  - [x] tz1bZ8vsMAXmaWEV7FRnyhcuUs2fYMaQ6Hkk - 746 - minimum not found
  - [ ] NOTE: waiting for confirmation from NL engineers (should be tomorrow)

- [x] rolling/archive mode
  - [x] configurable through configuration
  - [x] implement rolling mode - remove cycles older than L - 20 where L is the last completed cycle (default)
  - [x] implement archive mode - keep all cycles in the database

- [x] add consensus round delay - e.g. if 2 api returns for cycle 747 results of cycle 745 
  - [x] we should fetch it from network parameters when starting because it can be different on testnets
  
- [ ] configurable subset of delegates to track instead of all so people can run their mirrors with reduced overhead

- [ ] store test data in squashfs ( I'll do this)