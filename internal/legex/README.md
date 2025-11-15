# What

1. copy all the code from `regexp` (`regexp/syntax` is not included)
2. deleted all the unrelated code (like `Match...` and `Append...`)
3. make the `machine` able to preserve the matching position so it can be used for streaming data

> Surprisingly, I found that the regexp lib could actually support streaming data. See `match` function
> of `machine`, but regexp did not exploit this feature XD.

# TODO

[ ] Avoid duplicate thread add during multiple match
[ ] Add test case for submatch and gnarly regular expr
