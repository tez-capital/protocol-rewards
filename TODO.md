- [ ] fetching
  - [ ] fetch last completed cycle
  - [ ] fetch all delegates of specific cycle
    - [ ] make fetching parallel in batches, configurable batch size
      - NOTE: this would be done with channels and go routines
  - [ ] fetch states of all delegates of specific cycle
  - [ ] persists stats into database

- [ ] api 
  - [ ]  provide https://api.tzkt.io/v1/rewards/split/{baker}/{cycle}
	- NOTE: mirrors https://api.tzkt.io/#operation/Rewards_GetRewardSplit
  - [ ] api to manually trigger fetching of cycle
    - [ ] disablable from config
  - [ ] provide status api
    - [ ] last available cycle
  - [ ] for data not available return 404 not found 
  - [ ] 

- [ ] automatization
  - [ ] fetch every 5 minutes last finished cycle
  - [ ] check in database whether we have stats for this cycle
  - [ ] if not, fetch stats for this cycle
  - [ ] note in global state that fetching is in progress

- [ ] special cases
  - [ ] it may happen that when we lookup min balance for cycle X it wont be available and we have to fetch it from previous cycle
    - NOTE: we can detect it by checking that reported delegate.MinDelegated.Level.Level is 0
      right now it returns error, we should probably return specific error and handle properly
	  this is viable only for cycles above 744
