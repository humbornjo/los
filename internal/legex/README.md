# What

1. copy all the code from `regexp` (`depth = 1`: `regexp/syntax` is not included)
2. deleted all the unrelated code (like `Match...` and `Append...`)
3. make the `machine` able to preserve the matching position so it can be used for streaming data

> Surprisingly, I found that the regexp lib could actually support streaming data. See `match` function
> of `machine`, but regexp did not exploit this feature, It is more like they omit the resumable regex
> matching support. Yet there are some code I can steal XD.

# TODO

[x] Avoid duplicate thread add during multiple match on Machine
[ ] Add test case for submatch and gnarly regular expr
[ ] Integrate Onepass Machine and Backtrace Machine
