33039.2554954
72833.173514

744
105872

3 cycles delay -> affects cycle 747

works at - tz1aJHKKUWrwfsuoftdmwNBbBctjSWchMWZY - 5777367



what to ask to NL engineers?

- how to calculate staking
- which balance updates are irrelevant as burned
- is minimum triggered even if amount equals minimum or just if it is lower
- milligas rounding related? `2024/06/15 14:16:06 ERROR failed to find the exact balance delegate=tz1bZ8vsMAXmaWEV7FRnyhcuUs2fYMaQ6Hkk level_info="{Level:5795166 LevelPosition:5795165 Cycle:745 CyclePosition:19805 ExpectedCommitment:false VotingPeriod:0 VotingPeriodPosition:0}" target=324552206867 actual=324552206866`
- frozen deposits - actual vs initial amount?
- wtf `ERROR failed to find the exact balance delegate=tz1ZY5ug2KcAiaVfxhDKtKLx8U5zEgsxgdjV level_info="{Level:5799936 LevelPosition:5799935 Cycle:745 CyclePosition:24575 ExpectedCommitment:true VotingPeriod:0 VotingPeriodPosition:0}" target=121801316358 actual=128780046786`
- looks like freezer deposits are ignored

it may happen that when we lookup min balance for cycle X it wont be available and we have to fetch it from previous cycle???
- overstake from previous block or current?
- is overstake caused by frozen deposits or other fields?


tz1Q1GmjPwVxjxn81WMHmzdoFGjQDnw691b2
tz1abmz7jiCV2GH2u81LRrGgAFFgvQgiDiaf

for blogpost:

- rights are now calculated for cycle N from cycle N - 3 consisting from two parts:
  - amount of stake at the end of cycle (last block)
  - minimum amount of delegated balance during cycle N - 3 
    - protocol as it process each transaction / balance updates checks if it is lower than minimum and if so it updates minimum and its position

- TzC protocol rewards
	- after cycle N - 3	ends it collects all rewards from cycle N - 3 and stores them in database
  	   - collection is done in parallel to reasonably load RPC. protocol rewards are expected to use its own RPC node (rolling mode is sufficient)\
	   - collecting all data takes around 2 hours for 1 cycle
    	   - this is caused mostly by large bakers like everstake when we have to fetch ~60k delegators
    	   - because we pay rewards after cycle ends th actual few hours processing is ok. We have to wait for rewards to be paid anyway
        	   - we have always data ready at least 2 cycles in advance
	- it is possible to self host protocol rewards
       - supports rolling mode - only last N cycles are stored in database
       - allows selecting which bakers to store

    - returns results in format compatible with TZKT:
		```json
		{
			"cycle":748,
			"ownDelegatedBalance":18141930,
			"ownStakedBalance":11002954055,
			"externalDelegatedBalance":34705085017,
			"externalStakedBalance":4212539346,
			"delegatorsCount":48,
			"delegators": [
				{ 
					"address":"tz1Qzov1LwEvSfHonRHr2EwQGto7ExrH43or",
					"delegatedBalance":960009,
					"stakedBalance":0,
					// balance from stakedBalance includded in delegatedBalance
					"overstakedBalance": 0,
				}, ... 
			] }
		```