<h1>Performance</h1>

Поверку данных можно воспроизвести:
```
go test . -bench=. -benchmem -cpuprofile=cpu.out -memprofile=mem.out
```

Зависимость процессинга входных json данных от их объема носит линейный характер.

До оптимизации (коммит e9b7c8be от 15.04.22)

| Input, bytes | Time, ns/op      | Memory, b/op  | Allocs/op |
|--------------|------------------|---------------|-----------|
| 0            | 28               | 0             | 0         |
| 170          | 7148             | 1051          | 13        |
| 17000        | 772310  (0,77ms) | 106028        | 528       |
| 170000       | 9141206 (9ms)    | 840154        | 5042      |

После оптимизации (коммит 6487f446 от 18.04.22)

| Input, bytes | Time, ns/op       | Memory, b/op | Allocs/op |
|--------------|-------------------|--------------|-----------|
| 0            | 28                | 0            | 0         |
| 170          | 7883              | 641          | 10        |
| 17000        | 959341  (0,9ms)   | 61619        | 420       |
| 170000       | 10935865 (10,9ms) | 541358       | 4029      |

После внедрения пула буфферов уменьшился размер используемой памяти.

| Input, bytes | Time, ns/op       | Memory, b/op | Allocs/op |
|--------------|-------------------|--------------|-----------|
| 0            | 28                | 0            | 0         |
| 170          | 155965            | 489          | 10        |
| 17000        | 891860  (0,8ms)   | 39118        | 420       |
| 170000       | 10730853 (10,7ms) | 430186       | 4028      |

<h2>CPU Performance</h2>

Производительность по CPU на 88% зависит от скорости поиска по регулярному выражению,
а конкретно от метода `re.FindSubmatchIndex` и ниже по стеку.

```
$ go tool pprof cpu.out
Showing nodes accounting for 4.73s, 92.38% of 5.12s total
Dropped 74 nodes (cum <= 0.03s)
Showing top 15 nodes out of 63
      flat  flat%   sum%        cum   cum%
     1.33s 25.98% 25.98%      2.33s 45.51%  regexp.(*Regexp).tryBacktrack
     1.04s 20.31% 46.29%      1.24s 24.22%  regexp.(*machine).add
     0.41s  8.01% 54.30%      0.41s  8.01%  regexp.(*bitState).shouldVisit (inline)
     0.40s  7.81% 62.11%      0.68s 13.28%  regexp.(*machine).step
     0.39s  7.62% 69.73%      0.39s  7.62%  regexp.(*inputBytes).step
     0.27s  5.27% 75.00%      0.35s  6.84%  regexp.(*bitState).push (inline)
     0.21s  4.10% 79.10%      1.97s 38.48%  regexp.(*machine).match
     0.17s  3.32% 82.42%      0.17s  3.32%  runtime.memclrNoHeapPointers
     0.14s  2.73% 85.16%      2.72s 53.12%  regexp.(*Regexp).backtrack
     0.12s  2.34% 87.50%      0.12s  2.34%  runtime.memmove
     0.09s  1.76% 89.26%      0.09s  1.76%  regexp.(*machine).alloc
     0.08s  1.56% 90.82%      0.08s  1.56%  regexp/syntax.(*Inst).MatchRunePos
     0.03s  0.59% 91.41%      0.03s  0.59%  encoding/json.(*encodeState).string
     0.03s  0.59% 91.99%      0.03s  0.59%  sync.(*Pool).Put
     0.02s  0.39% 92.38%      0.09s  1.76%  github.com/cyberstudio/jsonj.doPassBatch.func1

```

<h2>Memory performance</h2>

В пакете стоит уделить внимание всему стеку вызовов в expandDataFragments и ниже.

```
(pprof) top 20 -cum -bytes -runtime gitlab
Active filters:
   focus=gitlab
   ignore=bytes|runtime
Showing nodes accounting for 32.02MB, 35.91% of 89.18MB total
      flat  flat%   sum%        cum   cum%
    0.50MB  0.56%  0.56%    32.02MB 35.91%  github.com/cyberstudio/jsonj.BenchmarkProcess
         0     0%  0.56%    31.52MB 35.35%  github.com/cyberstudio/jsonj.Process
         0     0%  0.56%    31.52MB 35.35%  github.com/cyberstudio/jsonj.doPassBatch
    2.50MB  2.80%  3.36%    24.02MB 26.93%  github.com/cyberstudio/jsonj.iterateMarks
      14MB 15.70% 19.07%       14MB 15.70%  github.com/cyberstudio/jsonj.doPassBatch.func1
         0     0% 19.07%     7.52MB  8.43%  regexp.(*Regexp).FindSubmatchIndex
       3MB  3.36% 22.43%     7.52MB  8.43%  regexp.(*Regexp).doExecute
       4MB  4.49% 26.92%     4.52MB  5.06%  regexp.(*Regexp).backtrack
       4MB  4.49% 31.40%        4MB  4.49%  encoding/json.Marshal
         0     0% 31.40%        4MB  4.49%  github.com/cyberstudio/jsonj.(*fragEntry).writeForReplaceValueMode
         0     0% 31.40%        4MB  4.49%  github.com/cyberstudio/jsonj.expandDataFragments
```
