Explanation[App{Benchmark(type:sum)}]

Func_pub[main{
  receive{},
  Var[let{int(i:0)}],
  Var[let{int(sum:0)}],
  Loop[for{int(i:0), lesseq(i:100000000), step{1}},
    Body[
      Mutation[variable{int(sum:+{int(sum:i)})}]
    ]
  ],
  return(sum)
}]
