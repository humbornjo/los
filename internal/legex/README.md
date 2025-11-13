# What

1. copy all the code from `regexp` (`regexp/syntax` is not included)
2. deleted all the unrelated code (like `Match...` and `Append...`)
3. make the `machine` able to preserve the matching position so it can be used for streaming data

> Surprisingly, I found that the regexp lib actually support streaming data. See `match` function
> of `machine`, but regexp did not exploit this feature XD.

# How

1. add a int field in `machine` to store the matching position
