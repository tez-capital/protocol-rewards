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